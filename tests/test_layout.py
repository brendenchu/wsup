import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

SOURCE = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(SOURCE / "src"))
from wsup_core import configuration, guidance, scaffolding


class LayoutTest(unittest.TestCase):
    def test_obsolete_bundled_home_readmes_retire_without_removing_user_homes(self):
        root = self.legacy()
        state_path = root / scaffolding.STATE
        state = json.loads(state_path.read_text())
        for home in ("apps", "infrastructure"):
            name = f"tools/wsup_templates/workspace/{home}/README.md"
            body = b"Historical collection guidance\n"
            scaffolding.atomic_write(root / name, body)
            state["files"][name] = scaffolding.digest(body)
            scaffolding.atomic_write(root / home / "README.md", b"Owner's collection\n")
        state_path.write_text(json.dumps(state))
        before = self.snapshot(root)
        preview = scaffolding.update(root)
        self.assertFalse(preview["conflicts"])
        self.assertEqual(before, self.snapshot(root))
        scaffolding.update(root, apply=True)
        for home in ("apps", "infrastructure"):
            self.assertEqual(b"Owner's collection\n", (root / home / "README.md").read_bytes())
            self.assertFalse((root / ".wsup/templates/workspace" / home / "README.md").exists())
        self.assertFalse(scaffolding.update(root)["actions"])

    def test_customized_obsolete_catalogue_readme_blocks_retirement(self):
        root = self.legacy()
        state_path = root / scaffolding.STATE
        state = json.loads(state_path.read_text())
        name = "tools/wsup_templates/workspace/apps/README.md"
        state["files"][name] = scaffolding.digest(b"Original guidance\n")
        scaffolding.atomic_write(root / name, b"Customized guidance\n")
        state_path.write_text(json.dumps(state))
        before = self.snapshot(root)
        self.assertIn(name, scaffolding.update(root)["conflicts"])
        with self.assertRaises(ValueError):
            scaffolding.update(root, apply=True)
        self.assertEqual(before, self.snapshot(root))

    def test_explicit_home_omissions_survive_updates_without_affecting_generation(self):
        root = self.root / "workspace"
        scaffolding.initialise(root)
        state_path = root / scaffolding.STATE
        state = json.loads(state_path.read_text())
        omitted = ["apps/.gitkeep", "packages/README.md"]
        state["omitted_home_files"] = omitted
        (root / omitted[0]).unlink()
        (root / omitted[0]).parent.rmdir()
        (root / omitted[1]).write_text("Keep existing content")
        state_path.write_text(json.dumps(state))
        scaffolding.update(root, apply=True)
        self.assertFalse((root / "apps").exists())
        self.assertEqual("Keep existing content", (root / omitted[1]).read_text())
        self.assertEqual(omitted, json.loads(state_path.read_text())["omitted_home_files"])
        self.assertFalse(scaffolding.update(root)["actions"])
        child = self.root / "child"
        result = subprocess.run(
            [sys.executable, "-B", str(root / ".wsup/wsup.py"), "init", str(child)],
            cwd=root, capture_output=True)
        self.assertEqual(0, result.returncode, result.stderr)
        for name in omitted:
            self.assertTrue((child / name).is_file())

    def test_invalid_home_omissions_block_all_update_writes(self):
        root = self.root / "workspace"
        scaffolding.initialise(root)
        state_path = root / scaffolding.STATE
        state = json.loads(state_path.read_text())
        for invalid in (None, "apps/.gitkeep", ["../escape"], [".wsup/wsup.py"], [1]):
            with self.subTest(invalid=invalid):
                state["omitted_home_files"] = invalid
                state_path.write_text(json.dumps(state))
                before = self.snapshot(root)
                with self.assertRaises(ValueError):
                    scaffolding.update(root, apply=True)
                self.assertEqual(before, self.snapshot(root))

    def test_development_sources_are_separate_from_generated_installation(self):
        self.assertEqual(SOURCE / "src/templates", scaffolding.catalogue())
        self.assertTrue((SOURCE / "src/wsup.py").is_file())
        self.assertTrue((SOURCE / ".wsup/template-state.json").is_file())
        self.assertFalse((SOURCE / "tools/wsup.py").exists())
        self.assertFalse((SOURCE / "tools/wsup_core").exists())
        self.assertTrue((SOURCE / "templates/embed.go").is_file())
        self.assertTrue((SOURCE / "templates/workspace/AI.md").is_file())
        self.assertNotEqual(SOURCE / "templates", scaffolding.catalogue())
        self.assertTrue((SOURCE / ".wsup/wsup.py").is_file())
        for name in ("apps", "packages", "tools", "infrastructure"):
            self.assertFalse((SOURCE / name).exists(), name)
        state = json.loads((SOURCE / ".wsup/template-state.json").read_text())
        self.assertEqual("workspace", state["template"])
        for name in (".wsup/wsup.py", ".wsup/wsup_core/scaffolding.py",
                     ".wsup/templates/workspace/AI.md"):
            self.assertIn(name, state["files"])
        for name, body in scaffolding.bundle("workspace").items():
            if name.startswith(".wsup/"):
                self.assertEqual(body, (SOURCE / name).read_bytes(), name)

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()

    def legacy(self, name="old", project=False):
        root = self.root / name
        root.mkdir()
        kind = "project" if project else "workspace"
        files = {}
        for path, body in scaffolding.bundle(kind).items():
            previous = path.replace(".wsup/templates/", "tools/wsup_templates/", 1)
            previous = previous.replace(".wsup/wsup_core/", "tools/wsup_core/", 1)
            if previous == ".wsup/wsup.py":
                previous = "tools/wsup.py"
            previous = previous.replace("guidance/rules/", ".ai/rules/")
            files[previous] = body
        if not project:
            for path, body in scaffolding.bundle("project").items():
                previous = path.replace("guidance/rules/", ".ai/rules/")
                files["templates/project/" + previous] = body
                files["tools/wsup_templates/workspace/templates/project/" + previous] = body
        for path, body in files.items():
            scaffolding.atomic_write(root / path, body)
        scaffolding.atomic_write(root / scaffolding.STATE, scaffolding.state_bytes(kind, files))
        return root

    def snapshot(self, root):
        return {path.relative_to(root): path.read_bytes()
                for path in root.rglob("*") if path.is_file()}

    def test_clean_legacy_migration_and_second_generation(self):
        root = self.legacy()
        report = b'{"path":"apps/old-location","recorded_at":"2020-01-01"}\n'
        scaffolding.atomic_write(root / ".work/quality/historical.json", report)
        scaffolding.atomic_write(root / ".work/investigation/raw.txt", b"Project-owned evidence")
        scaffolding.atomic_write(root / "apps/real-project/README.md", b"Retain this project")
        before = self.snapshot(root)
        preview = scaffolding.update(root)
        self.assertFalse(preview["conflicts"])
        self.assertEqual(before, self.snapshot(root))
        self.assertTrue(any(item["action"] == "retire" for item in preview["actions"]))
        self.assertTrue(any(item["action"] == "move-report" for item in preview["actions"]))
        scaffolding.update(root, apply=True)
        for path in ("templates", ".ai", ".work/quality"):
            self.assertFalse((root / path).exists(), path)
        for path in ("apps", "packages", "tools", "infrastructure", "workbench", "docs",
                     "guidance", ".agents/skills"):
            self.assertTrue((root / path).is_dir(), path)
        for path in scaffolding.COLLECTIONS:
            self.assertFalse((root / path).exists(), path)
        self.assertEqual(report, (root / ".wsup/reports/historical.json").read_bytes())
        self.assertTrue((root / ".work/investigation/raw.txt").is_file())
        self.assertTrue((root / "apps/real-project/README.md").is_file())
        self.assertFalse(guidance.inspect(root)["errors"])
        self.assertFalse(scaffolding.update(root)["actions"])
        self.assertFalse(scaffolding.update(root)["conflicts"])
        command = [sys.executable, "-B", str(root / ".wsup/wsup.py")]
        for action in ("validate", "guidance", "update"):
            result = subprocess.run(command + [action], cwd=root, capture_output=True)
            self.assertEqual(0, result.returncode, result.stderr)
        child = self.root / "child"
        result = subprocess.run(command + ["init", str(child)], cwd=root, capture_output=True)
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertFalse(guidance.inspect(child)["errors"])
        self.assertTrue((child / ".wsup/wsup.py").is_file())

    def test_workspace_starter_creates_opinionated_homes(self):
        root = self.root / "workspace"
        scaffolding.initialise(root)
        expected = ("apps", "packages", "tools", "infrastructure", "workbench", "docs",
                    "guidance/rules", ".agents/skills", ".wsup/templates")
        for path in expected:
            with self.subTest(path=path):
                self.assertTrue((root / path).is_dir())
        for path in scaffolding.COLLECTIONS:
            with self.subTest(path=path):
                self.assertFalse((root / path).exists())

    def test_update_adds_new_default_home_markers(self):
        root = self.root / "workspace"
        scaffolding.initialise(root)
        state_path = root / scaffolding.STATE
        state = json.loads(state_path.read_text())
        markers = ("apps/.gitkeep", "packages/README.md", "tools/README.md",
                   "infrastructure/.gitkeep", "workbench/README.md")
        for marker in markers:
            (root / marker).unlink()
            state["files"].pop(marker)
        state_path.write_text(json.dumps(state, indent=2) + "\n")

        preview = scaffolding.update(root)
        self.assertFalse(preview["conflicts"])
        self.assertEqual(
            set(markers),
            {item["path"] for item in preview["actions"] if item["action"] == "write"})
        scaffolding.update(root, apply=True)
        for marker in markers:
            self.assertTrue((root / marker).is_file(), marker)
        self.assertFalse(scaffolding.update(root)["actions"])

    def test_old_project_guidance_migrates_without_installing_runtime(self):
        root = self.legacy(project=True)
        self.assertFalse(scaffolding.update(root)["conflicts"])
        scaffolding.update(root, apply=True)
        self.assertFalse((root / ".ai").exists())
        self.assertFalse((root / ".wsup/wsup.py").exists())
        self.assertFalse(guidance.inspect(root)["errors"])

    def test_installed_catalogue_is_unambiguous_and_target_root_is_explicit(self):
        root = self.root / "installed"
        scaffolding.initialise(root)
        misleading = root / "templates/project/README.md"
        misleading.parent.mkdir(parents=True)
        misleading.write_text("This is not the installed catalogue")
        local_skill = root / ".agents/skills/workspace-check/SKILL.md"
        local_skill.write_text("A local customization, not a starter override")
        destination = self.root / "generated-project"
        result = subprocess.run(
            [sys.executable, "-B", str(root / ".wsup/wsup.py"),
             "init", str(destination), "--template", "project"],
            cwd=self.root, capture_output=True)
        self.assertEqual(0, result.returncode, result.stderr)
        workspace = self.root / "generated-workspace"
        result = subprocess.run(
            [sys.executable, "-B", str(root / ".wsup/wsup.py"), "init", str(workspace)],
            cwd=self.root, capture_output=True)
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertEqual(
            (SOURCE / "src/templates/workspace/.agents/skills/workspace-check/SKILL.md").read_bytes(),
            (workspace / ".agents/skills/workspace-check/SKILL.md").read_bytes())
        self.assertEqual(
            (SOURCE / "src/templates/project/README.md").read_bytes(),
            (destination / "README.md").read_bytes())
        result = subprocess.run(
            [sys.executable, "-B", str(root / ".wsup/wsup.py"),
             "--root", str(root), "validate"],
            cwd=self.root, capture_output=True)
        self.assertEqual(0, result.returncode, result.stderr)

    def test_conflicts_are_read_only_and_cannot_be_kept_away(self):
        for case in ("edited-runtime", "missing-rule", "collision", "unknown",
                     "edited-default-marker", "report-collision", "old-report-setting"):
            with self.subTest(case=case):
                root = self.legacy(case)
                if case == "edited-runtime":
                    (root / "tools/wsup.py").write_text("Local runtime change")
                elif case == "missing-rule":
                    (root / ".ai/rules/changes.md").unlink()
                elif case == "collision":
                    scaffolding.atomic_write(root / ".wsup/wsup.py", b"Existing destination")
                elif case == "unknown":
                    (root / ".ai/rules/custom.md").write_text("User-owned rule")
                elif case == "edited-default-marker":
                    (root / "packages/README.md").write_text("An actual package policy")
                elif case == "report-collision":
                    scaffolding.atomic_write(root / ".work/quality/run.json", b"old")
                    scaffolding.atomic_write(root / ".wsup/reports/run.json", b"new")
                else:
                    (root / "wsup.local.json").write_text(json.dumps({
                        "version": 1, "settings": {"report_dir": ".work/quality/custom"}}))
                before = self.snapshot(root)
                self.assertTrue(scaffolding.update(root)["conflicts"])
                with self.assertRaises(ValueError):
                    scaffolding.update(root, apply=True)
                self.assertEqual(before, self.snapshot(root))
        with self.assertRaisesRegex(ValueError, "--keep"):
            scaffolding.update(self.legacy("no-bypass"), keep=["tools/wsup.py"])

    def test_explicit_keep_preserves_custom_content_and_baseline(self):
        root = self.legacy()
        state = json.loads((root / scaffolding.STATE).read_text())
        registry = b'{"version":1,"name":"Personal","projects":[]}'
        (root / "workspace.json").write_bytes(registry)
        (root / "README.md").write_text("Personal workspace map")
        (root / "packages/README.md").write_text("Maintained collection")
        self.assertTrue(scaffolding.update(root)["conflicts"])
        keep = ["README.md", "packages/README.md"]
        self.assertFalse(scaffolding.update(root, keep=keep)["conflicts"])
        scaffolding.update(root, apply=True, keep=keep)
        self.assertEqual("Personal workspace map", (root / "README.md").read_text())
        self.assertEqual("Maintained collection", (root / "packages/README.md").read_text())
        self.assertEqual(registry, (root / "workspace.json").read_bytes())
        next_state = json.loads((root / scaffolding.STATE).read_text())
        self.assertEqual(state["files"]["README.md"], next_state["files"]["README.md"])
        self.assertEqual(state["files"]["packages/README.md"], next_state["files"]["packages/README.md"])
        self.assertIn("README.md", scaffolding.update(root)["conflicts"])
        self.assertEqual(
            ["packages/README.md"],
            scaffolding.update(root, keep=["README.md"])["conflicts"])
        self.assertFalse(scaffolding.update(root, keep=keep)["conflicts"])

    def test_migration_failure_preserves_old_sources_and_state(self):
        root = self.legacy()
        before = self.snapshot(root)
        with patch("wsup_core.scaffolding.atomic_write", side_effect=OSError("write denied")):
            with self.assertRaisesRegex(OSError, "write denied"):
                scaffolding.update(root, apply=True)
        self.assertEqual(before, self.snapshot(root))

    def test_report_containment_and_symlink_defenses(self):
        for path in (".work/quality", ".wsup/templates", "../reports", ".wsup/reports/../../escape"):
            with self.subTest(path=path):
                with self.assertRaises(ValueError):
                    configuration.load_config(self.root, {"report_dir": path}, environ={})
        values, _ = configuration.load_config(self.root, {"report_dir": ".wsup/reports/custom"}, environ={})
        self.assertEqual(".wsup/reports/custom", values["report_dir"])
        outside = self.root / "outside"
        outside.mkdir()
        (self.root / ".wsup").mkdir()
        (self.root / ".wsup/reports").symlink_to(outside, target_is_directory=True)
        with self.assertRaises(ValueError):
            configuration.load_config(self.root, environ={})
        (self.root / ".wsup/reports").unlink()
        (self.root / ".wsup").rmdir()
        (self.root / ".wsup").symlink_to(outside, target_is_directory=True)
        with self.assertRaises(ValueError):
            configuration.load_config(self.root, environ={})

    def test_symlinked_migration_destination_is_rejected(self):
        root = self.legacy()
        outside = self.root / "elsewhere"
        outside.mkdir()
        (root / ".wsup/reports").symlink_to(outside, target_is_directory=True)
        scaffolding.atomic_write(root / ".work/quality/run.json", b"Evidence")
        with self.assertRaises(ValueError):
            scaffolding.update(root, apply=True)
        self.assertFalse(any(outside.iterdir()))
        self.assertEqual(b"Evidence", (root / ".work/quality/run.json").read_bytes())


if __name__ == "__main__":
    unittest.main()
