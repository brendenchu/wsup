import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

SOURCE = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(SOURCE / "tools"))
from wsup_core import cli, configuration, execution, guidance, scaffolding

class ServicesTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        (self.root / "app").mkdir()
        (self.root / "app/README.md").write_text("Synthetic owner")
        self.registry = {"version": 1, "name": "Fixture", "projects": [{
            "id": "app", "path": "app", "purpose": "Synthetic", "owner": "Test",
            "kind": "app", "repository": "independent", "checks": {}}]}
        self.save()
    def save(self):
        (self.root / "workspace.json").write_text(json.dumps(self.registry))
    def invoke(self, *args):
        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
            return cli.main(["--root", str(self.root), *args])
    def checks(self, values):
        self.registry["projects"][0]["checks"] = values
        self.save()
    def reports(self):
        return [json.loads(p.read_text()) for p in (self.root / ".wsup/reports").glob("*.json")]
    def test_configuration_precedence_and_sources(self):
        for name, timeout in [("wsup.json", 20), ("wsup.local.json", 30)]:
            (self.root / name).write_text(json.dumps({"version": 1, "settings": {"timeout_seconds": timeout}}))
        values, sources = configuration.load_config(self.root, {"timeout_seconds": 50}, {"WSUP_TIMEOUT_SECONDS": "40"})
        self.assertEqual(50, values["timeout_seconds"])
        self.assertEqual("CLI", sources["timeout_seconds"])
        values, sources = configuration.load_config(self.root, environ={})
        self.assertEqual(30, values["timeout_seconds"])
        self.assertEqual("wsup.local.json", sources["timeout_seconds"])
    def test_bad_configuration_fails_before_command(self):
        self.checks({"write": [sys.executable, "-c", "open('unexpected','w').close()"]})
        for settings in [{"timeout_seconds": False}, {"report_dir": "../outside"}, {"unknown": 1}]:
            (self.root / "wsup.local.json").write_text(json.dumps({"version": 1, "settings": settings}))
            self.assertEqual(2, self.invoke("check", "--project", "app"))
            self.assertFalse((self.root / "app/unexpected").exists())
    def test_unknown_registry_fields_fail(self):
        self.registry["projects"][0]["typo"] = True
        self.save()
        self.assertEqual(2, self.invoke("validate"))
    def test_report_mode_finding_codes_and_required_failure(self):
        command = [sys.executable, "-c", "raise SystemExit(1)"]
        self.checks({"advisory": {"command": command, "mode": "report"}})
        self.assertEqual(0, self.invoke("check", "--project", "app"))
        self.assertEqual("findings", self.reports()[0]["results"][0]["status"])
        self.checks({"advisory": {"command": [sys.executable, "-c", "raise SystemExit(2)"], "mode": "report"}})
        self.assertEqual(1, self.invoke("check", "--project", "app"))
    def test_skips_are_explicit_and_required_skips_fail(self):
        self.checks({"skip": {"command": ["unused"], "enabled": False, "reason": "Unavailable fixture"}})
        self.assertEqual(1, self.invoke("check", "--project", "app"))
        self.assertEqual("skipped", self.reports()[0]["results"][0]["status"])
        self.registry["projects"][0]["checks"]["skip"]["reason"] = ""
        self.save()
        self.assertEqual(2, self.invoke("validate"))
    def test_missing_executable_is_error_even_in_report_mode(self):
        self.checks({"missing": {"command": [str(self.root / "missing-executable")], "mode": "report"}})
        self.assertEqual(1, self.invoke("check", "--project", "app"))
        self.assertEqual("error", self.reports()[0]["results"][0]["status"])
    def test_reports_omit_output_arguments_and_environment(self):
        secret = "SYNTHETIC-SECRET-CANARY"
        self.checks({"echo": [sys.executable, "-c", "import sys;print(sys.argv[1])", secret]})
        with patch.dict(os.environ, {"SYNTHETIC_PRIVATE_TOKEN": secret}):
            self.assertEqual(0, self.invoke("check", "--project", "app"))
        report = self.reports()[0]
        self.assertNotIn(secret, json.dumps(report))
        self.assertNotIn("stdout", report["results"][0])
        self.assertTrue(report["selection"][0]["command_sha256"])
        self.assertIn("workspace.json", report["inputs"])
    def test_large_output_is_bounded(self):
        result = execution.run([sys.executable, "-c", "import sys;sys.stdout.write('x'*200000);sys.stderr.write('y'*200000)"], self.root, 5, 128)
        self.assertEqual("passed", result["status"])
        self.assertEqual(128, len(result["stdout"]))
        self.assertEqual(400000, result["output_bytes"])
        self.assertTrue(result["output_truncated"])
    def test_timeout_is_distinct_from_failure(self):
        result = execution.run([sys.executable, "-c", "import time;time.sleep(5)"], self.root, 0.1, 128)
        self.assertEqual("timeout", result["status"])
        self.assertLess(result["duration_seconds"], 3)
    def test_cancellation_records_remaining_checks_as_skipped(self):
        self.checks({"first": ["unused"], "second": ["unused"]})
        with patch("wsup_core.cli.run", return_value={"status": "cancelled", "exit_code": None, "stdout": "", "stderr": ""}):
            self.assertEqual(130, self.invoke("check", "--project", "app"))
        self.assertEqual(["cancelled", "skipped"], [r["status"] for r in self.reports()[0]["results"]])
    @unittest.skipUnless(os.name == "posix", "POSIX process-group signal test")
    def test_actual_interrupt_terminates_check_and_reports(self):
        self.checks({"sleep": [sys.executable, "-c", "import time;open('started','w').close();time.sleep(30)"]})
        process = subprocess.Popen([sys.executable, "-B", str(SOURCE / "tools/wsup.py"), "--root", str(self.root), "check", "--project", "app"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        self.addCleanup(lambda: process.kill() if process.poll() is None else None)
        deadline = time.monotonic() + 5
        while not (self.root / "app/started").exists() and process.poll() is None and time.monotonic() < deadline:
            time.sleep(0.02)
        self.assertTrue((self.root / "app/started").exists())
        process.send_signal(signal.SIGINT)
        self.assertEqual(130, process.wait(timeout=5))
        self.assertEqual("cancelled", self.reports()[0]["results"][0]["status"])
    def test_guidance_detects_broken_local_reference(self):
        (self.root / "AI.md").write_text("[missing](missing.md)")
        (self.root / "guidance/rules").mkdir(parents=True)
        (self.root / "guidance/rules/index.md").write_text("Local rules")
        self.assertTrue(guidance.inspect(self.root)["errors"])
    def test_guidance_inspects_shared_skills_and_reports_legacy_coexistence(self):
        scaffolding.initialise(self.root / "workspace")
        root = self.root / "workspace"
        skill = root / ".agents/skills/workspace-check/SKILL.md"
        original = skill.read_text()
        result = guidance.inspect(root)
        self.assertFalse(result["errors"])
        self.assertEqual(12, len(result["skills"]))
        skill.write_text(original + "\n[missing](missing.md)\n")
        self.assertTrue(guidance.inspect(root)["errors"])
        skill.write_text(original.replace("name: workspace-check", "name: wrong-name"))
        self.assertTrue(guidance.inspect(root)["errors"])
        skill.write_text(original)
        legacy = root / ".ai/skills/workspace-check/SKILL.md"
        legacy.parent.mkdir(parents=True)
        legacy.write_text(original)
        result = guidance.inspect(root)
        self.assertEqual(12, len(result["skills"]))
        self.assertTrue(any("Legacy skills" in error for error in result["errors"]))
    def test_guidance_rejects_shared_skill_link_outside_owner(self):
        root = self.root / "workspace"
        scaffolding.initialise(root)
        skill = root / ".agents/skills/workspace-check/SKILL.md"
        skill.write_text(skill.read_text() + "\n[outside](../../../../workspace.json)\n")
        self.assertTrue(guidance.inspect(root)["errors"])
    def test_project_scaffold_is_standalone(self):
        target = self.root / "standalone"
        scaffolding.initialise(target, "project")
        self.assertFalse(guidance.inspect(target)["errors"])
        self.assertTrue((target / "README.md").exists())
        self.assertFalse((target / "workspace.json").exists())
    def test_generated_cli_can_generate_another_workspace(self):
        first, second = self.root / "first", self.root / "second"
        scaffolding.initialise(first)
        result = subprocess.run([sys.executable, "-B", ".wsup/wsup.py", "init", str(second)], cwd=first, capture_output=True)
        self.assertEqual(0, result.returncode, result.stderr)
        result = subprocess.run([sys.executable, "-B", ".wsup/wsup.py", "guidance"], cwd=second, capture_output=True)
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertNotIn(str(SOURCE), (second / "README.md").read_text())
    def test_update_preview_and_apply_preserve_customisations(self):
        target = self.root / "workspace"
        scaffolding.initialise(target)
        # Simulate an older template baseline while retaining a tracked user registry.
        state_path = target / scaffolding.STATE
        state = json.loads(state_path.read_text())
        (target / "docs/README.md").write_text("Older template")
        from wsup_core.reporting import digest
        state["files"]["docs/README.md"] = digest(b"Older template")
        state_path.write_text(json.dumps(state))
        (target / "workspace.json").write_text(json.dumps({"version":1,"name":"Custom","projects":[]}))
        before = (target / "docs/README.md").read_bytes()
        preview = scaffolding.update(target)
        self.assertIn({"path":"docs/README.md", "action":"write"}, preview["actions"])
        self.assertEqual(before, (target / "docs/README.md").read_bytes())
        scaffolding.update(target, apply=True)
        self.assertNotEqual(before, (target / "docs/README.md").read_bytes())
        self.assertEqual("Custom", json.loads((target / "workspace.json").read_text())["name"])
    def test_update_conflict_prevents_all_writes(self):
        target = self.root / "workspace"
        scaffolding.initialise(target)
        (target / "AI.md").write_text("User customisation")
        state = (target / scaffolding.STATE).read_bytes()
        preview = scaffolding.update(target)
        self.assertIn("AI.md", preview["conflicts"])
        with self.assertRaises(ValueError):
            scaffolding.update(target, apply=True)
        self.assertEqual(state, (target / scaffolding.STATE).read_bytes())
        self.assertEqual("User customisation", (target / "AI.md").read_text())
    def test_manifest_path_escape_rejected(self):
        target = self.root / "workspace"
        scaffolding.initialise(target)
        state_path = target / scaffolding.STATE
        state = json.loads(state_path.read_text())
        state["files"]["../escape"] = "fake"
        state_path.write_text(json.dumps(state))
        with self.assertRaises(ValueError):
            scaffolding.update(target)
    def legacy_workspace(self, name="legacy"):
        target = self.root / name
        scaffolding.initialise(target)
        state_path = target / scaffolding.STATE
        state = json.loads(state_path.read_text())
        from wsup_core.reporting import digest
        for prefix in ("", ".wsup/templates/workspace/"):
            current = target / prefix / ".agents/skills"
            legacy = target / prefix / ".ai/skills"
            legacy.parent.mkdir(parents=True, exist_ok=True)
            current.rename(legacy)
            for path in list(state["files"]):
                if path.startswith(prefix + ".agents/skills/"):
                    previous = path.replace(".agents/skills/", ".ai/skills/", 1)
                    legacy_file = target / previous
                    legacy_file.write_text(legacy_file.read_text().replace(".agents/skills/", ".ai/skills/"))
                    state["files"].pop(path)
                    state["files"][previous] = digest(legacy_file.read_bytes())
        state["template_version"] = "0.1.0"
        state_path.write_text(json.dumps(state))
        return target
    def test_legacy_skills_migrate_with_preview_and_repeatable_apply(self):
        target = self.legacy_workspace()
        state_path = target / scaffolding.STATE
        before = state_path.read_bytes()
        (target / "workspace.json").write_text('{"version":1,"name":"Custom","projects":[]}')
        preview = scaffolding.update(target)
        self.assertFalse(preview["conflicts"])
        moves = [item for item in preview["actions"] if item["action"] == "relocate"]
        self.assertEqual(24, len(moves))
        self.assertEqual(before, state_path.read_bytes())
        self.assertTrue((target / ".ai/skills/workspace-check/SKILL.md").exists())
        self.assertFalse((target / ".agents/skills/workspace-check/SKILL.md").exists())
        scaffolding.update(target, apply=True)
        self.assertFalse(list((target / ".ai/skills").rglob("*.*")))
        self.assertTrue((target / ".agents/skills/workspace-check/SKILL.md").is_file())
        self.assertFalse(guidance.inspect(target)["errors"])
        self.assertFalse(scaffolding.update(target)["actions"])
        self.assertFalse(scaffolding.update(target)["conflicts"])
        self.assertEqual("Custom", json.loads((target / "workspace.json").read_text())["name"])
        state = json.loads(state_path.read_text())
        self.assertFalse(any(".ai/skills/" in name for name in state["files"]))
    def test_legacy_migration_conflicts_preserve_all_files(self):
        for case in ("edited", "missing", "collision", "unrecognized", "unmanaged", "embedded-edit"):
            with self.subTest(case=case):
                target = self.legacy_workspace(case)
                legacy = target / ".ai/skills/workspace-check/SKILL.md"
                if case == "edited":
                    legacy.write_text(legacy.read_text() + "\nUser edits\n")
                elif case == "missing":
                    legacy.unlink()
                elif case == "collision":
                    destination = target / ".agents/skills/workspace-check/SKILL.md"
                    destination.parent.mkdir(parents=True)
                    destination.write_bytes(legacy.read_bytes())
                elif case == "unrecognized":
                    (legacy.parent / "notes.txt").write_text("User notes")
                elif case == "embedded-edit":
                    embedded = target / ".wsup/templates/workspace/.ai/skills/workspace-check/SKILL.md"
                    embedded.write_text("User template edit")
                else:
                    state_path = target / scaffolding.STATE
                    state = json.loads(state_path.read_text())
                    del state["files"][".ai/skills/workspace-check/SKILL.md"]
                    state_path.write_text(json.dumps(state))
                before = {p.relative_to(target): p.read_bytes() for p in target.rglob("*") if p.is_file()}
                self.assertTrue(scaffolding.update(target)["conflicts"])
                with self.assertRaises(ValueError):
                    scaffolding.update(target, apply=True)
                self.assertEqual(before, {
                    p.relative_to(target): p.read_bytes() for p in target.rglob("*") if p.is_file()})
    def test_legacy_migration_failure_keeps_original_skills(self):
        target = self.legacy_workspace()
        state = (target / scaffolding.STATE).read_bytes()
        original = {p.relative_to(target): p.read_bytes() for p in (target / ".ai/skills").rglob("SKILL.md")}
        with patch("wsup_core.scaffolding.atomic_write", side_effect=OSError("Simulated write failure")):
            with self.assertRaisesRegex(OSError, "Simulated write failure"):
                scaffolding.update(target, apply=True)
        self.assertEqual(state, (target / scaffolding.STATE).read_bytes())
        for name, contents in original.items():
            self.assertEqual(contents, (target / name).read_bytes())
    def test_update_rechecks_state_before_writing(self):
        target = self.legacy_workspace()
        real_bundle = scaffolding.bundle
        def change_state(kind, collections=()):
            state_path = target / scaffolding.STATE
            state_path.write_text(state_path.read_text() + "\n")
            return real_bundle(kind, collections)
        with patch("wsup_core.scaffolding.bundle", side_effect=change_state):
            with self.assertRaisesRegex(ValueError, "state changed"):
                scaffolding.update(target, apply=True)
        self.assertFalse((target / ".agents/skills/workspace-check/SKILL.md").exists())
    def test_newer_cli_upgrades_legacy_workspace_then_runs_standalone(self):
        target = self.legacy_workspace()
        state_path = target / scaffolding.STATE
        before = state_path.read_bytes()
        command = [sys.executable, "-B", str(SOURCE / "tools/wsup.py"),
                   "--root", str(target), "update"]
        preview = subprocess.run(command, capture_output=True, text=True)
        self.assertEqual(0, preview.returncode, preview.stderr)
        result = json.loads(preview.stdout)
        self.assertEqual("0.1.0", result["from_version"])
        self.assertEqual("0.1.0", result["to_version"])
        self.assertFalse(result["applied"])
        self.assertFalse(result["conflicts"])
        self.assertEqual(24, len([item for item in result["actions"] if item["action"] == "relocate"]))
        self.assertEqual(before, state_path.read_bytes())
        applied = subprocess.run(command + ["--apply"], capture_output=True, text=True)
        self.assertEqual(0, applied.returncode, applied.stderr)
        self.assertTrue(json.loads(applied.stdout)["applied"])
        for action in ("guidance", "validate", "update"):
            result = subprocess.run(
                [sys.executable, "-B", ".wsup/wsup.py", action],
                cwd=target, capture_output=True, text=True)
            self.assertEqual(0, result.returncode, result.stderr)
            if action == "update":
                self.assertEqual([], json.loads(result.stdout)["actions"])
        self.assertNotIn(str(SOURCE), (target / ".wsup/wsup_core/scaffolding.py").read_text())
    def test_legacy_migration_refuses_symlink_sources(self):
        target = self.legacy_workspace()
        legacy = target / ".ai/skills/workspace-check/SKILL.md"
        contents = legacy.read_bytes()
        legacy.unlink()
        source = self.root / "source.md"
        source.write_bytes(contents)
        try:
            legacy.symlink_to(source)
        except (OSError, NotImplementedError):
            self.skipTest("Symlink creation unavailable")
        with self.assertRaises(ValueError):
            scaffolding.update(target, apply=True)
        self.assertTrue(legacy.is_symlink())
        self.assertEqual(contents, source.read_bytes())

if __name__ == "__main__":
    unittest.main()
