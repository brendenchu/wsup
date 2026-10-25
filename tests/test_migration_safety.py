"""Black-box migration safety probes; optionally run against an archived source."""
import argparse
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
import unittest

from migration_baseline import SOURCE, snapshot


class MigrationSafetyTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="wsup-safety-")
        self.addCleanup(temporary.cleanup)
        self.base = Path(temporary.name).resolve()
        self.root = self.base / "workspace"
        self.cli = SOURCE / "src/wsup.py"

    def invoke(self, *args, code=0):
        env = {key: value for key, value in os.environ.items()
               if not key.startswith("WSUP_")}
        result = subprocess.run(
            [sys.executable, "-B", str(self.cli), "--root", str(self.root), *args],
            cwd=self.base, env=env, capture_output=True, text=True, timeout=15)
        self.assertEqual(code, result.returncode, result.stdout + result.stderr)
        return result.stdout

    def initialise(self):
        self.invoke("init", str(self.root))

    def test_omitted_homes_preserve_bytes_without_changing_child_generation(self):
        self.initialise()
        state_path = self.root / ".wsup/template-state.json"
        state = json.loads(state_path.read_text())
        omitted = ["apps/.gitkeep", "packages/README.md"]
        state["omitted_home_files"] = omitted
        (self.root / omitted[0]).unlink()
        (self.root / "apps").rmdir()
        body = b"Owner's package policy\x00\n"
        (self.root / omitted[1]).write_bytes(body)
        state_path.write_text(json.dumps(state))
        before = snapshot(self.root)
        preview = json.loads(self.invoke("update"))
        self.assertEqual([], preview["conflicts"])
        self.assertEqual(before, snapshot(self.root))
        self.invoke("update", "--apply")
        self.assertFalse((self.root / "apps").exists())
        self.assertEqual(body, (self.root / omitted[1]).read_bytes())
        updated = json.loads(state_path.read_text())
        self.assertEqual(omitted, updated["omitted_home_files"])
        self.assertEqual([], json.loads(self.invoke("update"))["actions"])
        child = self.base / "child"
        self.cli = self.root / ".wsup/wsup.py"
        self.invoke("init", str(child))
        self.assertTrue(all((child / name).is_file() for name in omitted))

    @unittest.skipUnless(os.name == "posix", "POSIX process-group contract")
    def test_interrupt_stops_descendant_and_skips_remaining_checks(self):
        self.check_process_tree(interrupt=True)

    @unittest.skipUnless(os.name == "posix", "POSIX process-group contract")
    def test_exited_parent_with_retained_pipes_does_not_hang(self):
        self.check_process_tree(interrupt=False)

    def check_process_tree(self, interrupt):
        (self.root / "app").mkdir(parents=True)
        (self.root / "app/README.md").write_text("Synthetic process fixture\n")
        child = (
            "import os,time\n"
            "from pathlib import Path\n"
            "Path('child.pid').write_text(str(os.getpid()))\n"
            "for i in range(200):\n"
            " Path('heartbeat').write_text(str(i));time.sleep(0.05)\n")
        parent = (
            "import subprocess,sys,time\n"
            "from pathlib import Path\n"
            f"subprocess.Popen([sys.executable,'-c',{child!r}])\n"
            "while not Path('heartbeat').exists(): time.sleep(0.01)\n"
            + ("time.sleep(10)\n" if interrupt else "print('parent exited',flush=True)\n"))
        registry = {"version": 1, "name": "Process fixture", "projects": [{
            "id": "app", "path": "app", "kind": "app", "repository": "workspace",
            "purpose": "Synthetic", "owner": "Test", "checks": {
                "tree": ["{python}", "-c", parent],
                "next": ["{python}", "-c", "open('unexpected','w').close()"]}}]}
        (self.root / "workspace.json").write_text(json.dumps(registry))
        env = {key: value for key, value in os.environ.items()
               if not key.startswith("WSUP_")}
        process = subprocess.Popen(
            [sys.executable, "-B", str(self.cli), "--root", str(self.root),
             "check", "--project", "app"],
            cwd=self.base, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            text=True)
        child_pid = None
        try:
            deadline = time.monotonic() + 5
            heartbeat = self.root / "app/heartbeat"
            while not heartbeat.exists() and process.poll() is None and time.monotonic() < deadline:
                time.sleep(0.01)
            self.assertTrue(heartbeat.exists(), "Child never signaled readiness")
            child_pid = int((self.root / "app/child.pid").read_text())
            if interrupt:
                process.send_signal(signal.SIGINT)
            stdout, stderr = process.communicate(timeout=5)
            self.assertEqual(130 if interrupt else 0, process.returncode, stdout + stderr)
            stopped = heartbeat.read_bytes()
            time.sleep(0.15)
            self.assertEqual(stopped, heartbeat.read_bytes(), "Descendant still running")
            self.assertEqual(not interrupt, (self.root / "app/unexpected").exists())
            reports = list((self.root / ".wsup/reports").glob("*.json"))
            self.assertEqual(1, len(reports))
            report = json.loads(reports[0].read_text())
            self.assertEqual(
                ["cancelled", "skipped"] if interrupt else ["passed", "passed"],
                [result["status"] for result in report["results"]])
            if interrupt:
                self.assertEqual("cancelled", report["results"][1]["reason"])
        finally:
            if process.poll() is None:
                process.kill()
            process.communicate(timeout=5)
            if child_pid is None:
                pid_path = self.root / "app/child.pid"
                if pid_path.exists():
                    child_pid = int(pid_path.read_text())
            if child_pid is not None:
                try:
                    os.kill(child_pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, default=SOURCE)
    args = parser.parse_args()
    SOURCE = args.source.resolve()
    unittest.main(argv=[sys.argv[0]], verbosity=2)
