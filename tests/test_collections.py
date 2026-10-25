import contextlib
import io
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

SOURCE = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(SOURCE / "tools"))
from wsup_core import cli, scaffolding


class CollectionsTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve() / "workspace"

    def state(self):
        return json.loads((self.root / scaffolding.STATE).read_text())

    def snapshot(self):
        return {p.relative_to(self.root).as_posix(): p.read_bytes()
                for p in self.root.rglob("*") if p.is_file()}

    def run_cli(self, *args):
        with contextlib.redirect_stdout(io.StringIO()) as output:
            with contextlib.redirect_stderr(io.StringIO()):
                code = cli.main(["--root", str(self.root), *args])
        return code, output.getvalue()

    def test_init_selects_only_requested_collections_and_bundles_catalogue(self):
        self.assertEqual(0, self.run_cli(
            "init", str(self.root), "--collection", "archive",
            "--collection", "private", "--collection", "archive")[0])
        self.assertEqual(["archive", "private"], self.state()["collections"])
        for name in scaffolding.COLLECTIONS:
            self.assertTrue((self.root / ".wsup/templates/collections" / name / "README.md").is_file())
            self.assertEqual(name in ("archive", "private"), (self.root / name / "README.md").is_file())
        self.assertFalse(scaffolding.update(self.root)["actions"])
        self.assertFalse(scaffolding.update(self.root)["conflicts"])

    def test_list_and_preview_are_read_only_and_apply_is_idempotent(self):
        scaffolding.initialise(self.root)
        before = self.snapshot()
        code, output = self.run_cli("collections")
        self.assertEqual(0, code)
        self.assertEqual(list(scaffolding.COLLECTIONS), json.loads(output)["available"])
        code, output = self.run_cli("collections", "--add", "artifacts")
        self.assertEqual(0, code)
        self.assertEqual([{"path": "artifacts/README.md", "action": "write"}],
                         json.loads(output)["actions"])
        self.assertEqual(before, self.snapshot())
        self.assertEqual(0, self.run_cli("collections", "--add", "artifacts", "--apply")[0])
        self.assertEqual(["artifacts"], self.state()["collections"])
        self.assertTrue((self.root / "artifacts/README.md").is_file())
        before = self.snapshot()
        self.assertEqual([], scaffolding.add_collections(self.root, ["artifacts"], True)["actions"])
        self.assertEqual(before, self.snapshot())

    def test_every_optional_collection_can_be_selected(self):
        scaffolding.initialise(self.root, collections=list(scaffolding.COLLECTIONS))
        for name in scaffolding.COLLECTIONS:
            self.assertTrue((self.root / name / "README.md").is_file(), name)
        self.assertTrue((self.root / "journal/TEMPLATE.md").is_file())
        self.assertFalse(scaffolding.update(self.root)["actions"])

    def test_previous_default_homes_are_preserved_and_can_be_selected(self):
        scaffolding.initialise(self.root)
        files = scaffolding.collection_files(["journal", "learning", "reference", "research"])
        state = self.state()
        for name, body in files.items():
            scaffolding.atomic_write(self.root / name, body)
            state["files"][name] = scaffolding.digest(body)
            old_catalogue = ".wsup/templates/workspace/" + name
            scaffolding.atomic_write(self.root / old_catalogue, body)
            state["files"][old_catalogue] = scaffolding.digest(body)
        (self.root / scaffolding.STATE).write_text(json.dumps(state))
        (self.root / "learning/README.md").write_text("Custom learning home")
        result = scaffolding.add_collections(
            self.root, ["journal", "learning", "reference", "research"], True)
        self.assertFalse(result["actions"])
        self.assertFalse(result["conflicts"])
        preview = scaffolding.update(self.root, keep=["learning/README.md"])
        self.assertFalse(preview["conflicts"])
        scaffolding.update(self.root, apply=True, keep=["learning/README.md"])
        self.assertEqual("Custom learning home", (self.root / "learning/README.md").read_text())
        self.assertFalse((self.root / ".wsup/templates/workspace/learning/README.md").exists())
        self.assertTrue((self.root / ".wsup/templates/collections/learning/README.md").is_file())

    def test_unselected_former_defaults_become_orphans_not_new_defaults(self):
        scaffolding.initialise(self.root)
        state = self.state()
        files = scaffolding.collection_files(["journal", "learning", "reference", "research"])
        for name, body in files.items():
            for path in (name, ".wsup/templates/workspace/" + name):
                scaffolding.atomic_write(self.root / path, body)
                state["files"][path] = scaffolding.digest(body)
        (self.root / scaffolding.STATE).write_text(json.dumps(state))
        (self.root / "journal/README.md").write_text("Local journal policy")
        scaffolding.update(self.root, apply=True)
        self.assertNotIn("collections", self.state())
        for name in files:
            self.assertTrue((self.root / name).is_file())
            self.assertNotIn(name, self.state()["files"])
            self.assertFalse((self.root / ".wsup/templates/workspace" / name).exists())
        self.assertEqual("Local journal policy", (self.root / "journal/README.md").read_text())
        child = Path(self.temp.name) / "child"
        result = subprocess.run(
            [sys.executable, "-B", str(self.root / ".wsup/wsup.py"), "init", str(child)],
            cwd=self.root, capture_output=True)
        self.assertEqual(0, result.returncode, result.stderr)
        for name in scaffolding.COLLECTIONS:
            self.assertFalse((child / name).exists())

    def test_customized_former_catalogue_blocks_retirement_without_writes(self):
        scaffolding.initialise(self.root)
        name = ".wsup/templates/workspace/journal/README.md"
        scaffolding.atomic_write(self.root / name, b"Original\n")
        state = self.state()
        state["files"][name] = scaffolding.digest(b"Original\n")
        (self.root / scaffolding.STATE).write_text(json.dumps(state))
        (self.root / name).write_text("Customized catalogue\n")
        before = self.snapshot()
        self.assertIn(name, scaffolding.update(self.root)["conflicts"])
        with self.assertRaises(ValueError):
            scaffolding.update(self.root, apply=True)
        self.assertEqual(before, self.snapshot())

    def test_invalid_state_and_missing_selected_file_fail_without_writes(self):
        scaffolding.initialise(self.root, collections=["archive"])
        state = self.state()
        for invalid in (None, "archive", ["unknown"], [1]):
            with self.subTest(invalid=invalid):
                state["collections"] = invalid
                (self.root / scaffolding.STATE).write_text(json.dumps(state))
                before = self.snapshot()
                for operation in (scaffolding.add_collections, scaffolding.update):
                    with self.assertRaises(ValueError):
                        if operation == scaffolding.add_collections:
                            operation(self.root, ["artifacts"], apply=True)
                        else:
                            operation(self.root, apply=True)
                self.assertEqual(before, self.snapshot())
        state["collections"] = ["archive"]
        (self.root / scaffolding.STATE).write_text(json.dumps(state))
        (self.root / "archive/README.md").unlink()
        before = self.snapshot()
        with self.assertRaises(ValueError):
            scaffolding.add_collections(self.root, ["archive", "artifacts"], apply=True)
        self.assertEqual(before, self.snapshot())

    def test_existing_files_block_entire_apply_and_are_not_adopted(self):
        scaffolding.initialise(self.root)
        (self.root / "archive").mkdir()
        (self.root / "archive/README.md").write_text("Owner's existing policy")
        before = self.snapshot()
        result = scaffolding.add_collections(self.root, ["archive", "artifacts"])
        self.assertEqual(["archive/README.md"], result["conflicts"])
        self.assertFalse(result["applied"])
        self.assertEqual(1, self.run_cli("collections", "--add", "archive")[0])
        self.assertEqual(2, self.run_cli(
            "collections", "--add", "archive", "--add", "artifacts", "--apply")[0])
        self.assertEqual(before, self.snapshot())
        self.assertFalse((self.root / "artifacts").exists())

    def test_existing_directory_contents_are_not_read_or_changed(self):
        scaffolding.initialise(self.root)
        (self.root / "archive").mkdir()
        record = self.root / "archive/existing.txt"
        record.write_text("Keep unchanged")
        scaffolding.add_collections(self.root, ["archive"], True)
        self.assertEqual("Keep unchanged", record.read_text())
        self.assertTrue((self.root / "archive/README.md").is_file())

    def test_selection_survives_updates_and_customizations_use_normal_conflicts(self):
        scaffolding.initialise(self.root, collections=["archive"])
        scaffolding.add_collections(self.root, ["artifacts"], True)
        scaffolding.update(self.root, apply=True)
        self.assertEqual(["archive", "artifacts"], self.state()["collections"])
        file = self.root / "archive/README.md"
        file.write_text("Custom archive policy")
        before = self.snapshot()
        self.assertEqual([], scaffolding.add_collections(self.root, ["archive"], True)["actions"])
        self.assertEqual(before, self.snapshot())
        self.assertIn("archive/README.md", scaffolding.update(self.root)["conflicts"])
        scaffolding.update(self.root, apply=True, keep=["archive/README.md"])
        self.assertEqual("Custom archive policy", file.read_text())

    def test_collection_scaffold_updates_with_catalogue(self):
        scaffolding.initialise(self.root, collections=["archive"])
        original = scaffolding.bundle

        def changed_bundle(kind, collections=()):
            files = original(kind, collections)
            files["archive/README.md"] = b"Updated archive guidance\n"
            return files

        with patch("wsup_core.scaffolding.bundle", side_effect=changed_bundle):
            self.assertIn({"path": "archive/README.md", "action": "write"},
                          scaffolding.update(self.root)["actions"])
            scaffolding.update(self.root, apply=True)
        self.assertEqual(b"Updated archive guidance\n", (self.root / "archive/README.md").read_bytes())
        self.assertEqual(["archive"], self.state()["collections"])

    def test_invalid_selection_project_template_and_unmanaged_root_fail_without_writes(self):
        with self.assertRaisesRegex(ValueError, "workspace template"):
            scaffolding.initialise(self.root, "project", ["archive"])
        self.assertFalse(self.root.exists())
        for invalid in (["unknown"], ["../escape"], "archive", None):
            with self.subTest(invalid=invalid):
                with self.assertRaises(ValueError):
                    scaffolding.initialise(self.root, collections=invalid)
                self.assertFalse(self.root.exists())
        self.assertEqual(2, self.run_cli("collections", "--add", "archive", "--apply")[0])
        self.assertFalse(self.root.exists())
        self.assertEqual(2, self.run_cli("collections", "--apply")[0])

    def test_symlinks_and_non_directory_parents_are_rejected(self):
        for case in ("directory-symlink", "file-symlink", "file-parent"):
            with self.subTest(case=case):
                root = Path(self.temp.name).resolve() / case
                scaffolding.initialise(root)
                outside = Path(self.temp.name).resolve() / (case + "-outside")
                outside.mkdir()
                if case == "directory-symlink":
                    (root / "archive").symlink_to(outside, target_is_directory=True)
                elif case == "file-symlink":
                    (root / "archive").mkdir()
                    (root / "archive/README.md").symlink_to(outside / "missing")
                else:
                    (root / "archive").write_text("Not a directory")
                before = (root / scaffolding.STATE).read_bytes()
                with self.assertRaises(ValueError):
                    scaffolding.add_collections(root, ["archive"], True)
                self.assertEqual(before, (root / scaffolding.STATE).read_bytes())
                self.assertEqual([], list(outside.iterdir()))

    def test_installed_cli_supports_opt_ins_and_second_generation(self):
        scaffolding.initialise(self.root)
        command = [sys.executable, "-B", str(self.root / ".wsup/wsup.py")]
        result = subprocess.run(command + ["collections", "--add", "archive", "--apply"],
                                cwd=self.root, capture_output=True)
        self.assertEqual(0, result.returncode, result.stderr)
        child = Path(self.temp.name) / "child"
        result = subprocess.run(command + [
            "init", str(child), "--collection", "artifacts"], cwd=self.root, capture_output=True)
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertTrue((child / "artifacts/README.md").is_file())
        self.assertFalse((child / "archive").exists())
        result = subprocess.run([sys.executable, "-B", ".wsup/wsup.py",
                                 "collections", "--add", "private", "--apply"],
                                cwd=child, capture_output=True)
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertTrue((child / "private/README.md").is_file())
