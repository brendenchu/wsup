"""Capture immutable Python contracts and replay them through a CLI subprocess."""
import argparse
import ast
import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tarfile
import tempfile

SOURCE = Path(__file__).resolve().parents[1]
DATA = SOURCE / "testdata/python-baseline"
BASELINE = "3b709d522ddd1aab38c4442b20cfb22eb1787915"
REPORT = re.compile(r"\d{8}T\d{6}-[0-9a-f]{12}")


def archive(commit, destination):
    raw = subprocess.check_output(["git", "archive", commit], cwd=SOURCE)
    with tarfile.open(fileobj=io.BytesIO(raw)) as contents:
        for member in contents.getmembers():
            target = (destination / member.name).resolve()
            if not target.is_relative_to(destination.resolve()) or member.issym() or member.islnk():
                raise ValueError(f"Unsafe archive member: {member.name}")
        contents.extractall(destination)


def normalize(value, paths):
    if isinstance(value, str):
        for path, label in paths:
            value = value.replace(str(path), label)
        return REPORT.sub("$REPORT", value)
    if isinstance(value, list):
        return [normalize(item, paths) for item in value]
    if isinstance(value, dict):
        return {normalize(key, paths): normalize(item, paths) for key, item in value.items()}
    return value


def snapshot(root):
    result = {}
    for path in sorted(root.rglob("*")):
        if path.is_symlink():
            result[path.relative_to(root).as_posix()] = {"symlink": os.readlink(path)}
        elif path.is_file():
            body = path.read_bytes()
            result[path.relative_to(root).as_posix()] = {
                "sha256": hashlib.sha256(body).hexdigest(), "bytes": len(body)}
        elif path.is_dir():
            result[path.relative_to(root).as_posix() + "/"] = {"directory": True}
    return result


def output(text, paths):
    try:
        value = json.loads(text)
    except json.JSONDecodeError:
        value = text
    return normalize(value, paths)


def report_files(root, paths):
    result = {}
    for path in sorted((root / ".wsup/reports").glob("*.json")):
        value = json.loads(path.read_text())
        value["recorded_at"] = "$TIME"
        value["runtime"] = "$RUNTIME"
        for item in value["results"]:
            if "duration_seconds" in item:
                item["duration_seconds"] = "$DURATION"
        result[path.stem] = normalize(value, paths)
    return result


def set_field(document, dotted, value=None, delete=False):
    parts = dotted.split(".")
    target = document
    for part in parts[:-1]:
        target = target[int(part)] if isinstance(target, list) else target[part]
    key = int(parts[-1]) if isinstance(target, list) else parts[-1]
    if delete:
        del target[key]
    else:
        target[key] = value


def replay(command, compare_read_only=None):
    cases = json.loads((DATA / "cases.json").read_text())["cases"]
    results = {}
    for case in cases:
        with tempfile.TemporaryDirectory(prefix="wsup-contract-") as temporary:
            base = Path(temporary).resolve()
            root = base / "workspace"
            paths = [(root, "$ROOT"), (base, "$CWD")]
            if not case.get("generated"):
                (root / "app").mkdir(parents=True)
                (root / "app/README.md").write_text("Synthetic project\n")
                registry = {"version": 1, "name": "Synthetic", "projects": [{
                    "id": "app", "path": "app", "kind": "app",
                    "repository": "independent", "purpose": "Fixture",
                    "owner": "Test", "checks": {}}]}
                (root / "workspace.json").write_text(json.dumps(registry))
            records = []
            for step in case["steps"]:
                for name in step.get("remove", []):
                    (root / name).unlink()
                for name, destination in step.get("symlinks", {}).items():
                    (root / name).symlink_to(base / destination, target_is_directory=True)
                for name, body in step.get("files", {}).items():
                    target = root / name
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_text(body)
                if "set" in step or "delete" in step:
                    document = root / step.get("document", "workspace.json")
                    registry = json.loads(document.read_text())
                    for field in step.get("delete", []):
                        set_field(registry, field, delete=True)
                    for field, value in step.get("set", {}).items():
                        set_field(registry, field, value)
                    document.write_text(json.dumps(registry))
                before = snapshot(base)
                old_reports = report_files(root, paths)
                args = [arg.replace("$ROOT", str(root)) for arg in step["args"]]
                env = {key: value for key, value in os.environ.items()
                       if not key.startswith("WSUP_")}
                env.update(step.get("env", {}))
                env["PYTHONDONTWRITEBYTECODE"] = "1"
                invoked = command + (["--root", str(root)] if step.get("root", True) else []) + args
                editor_result = None
                editor_after = None
                if compare_read_only and step["args"][0] == "editor":
                    editor_result = subprocess.run(
                        compare_read_only + ["--root", str(root)] + args,
                        cwd=base, env=env, capture_output=True, text=True, timeout=20)
                    editor_after = snapshot(base)
                    if any(editor_after.get(name) != body for name, body in before.items()):
                        raise AssertionError("Go editor changed existing files")
                    for name in set(editor_after) - set(before):
                        target = base / name
                        if not target.is_file() or target.is_symlink():
                            raise AssertionError(f"Unexpected Go editor output: {name}")
                        target.unlink()
                completed = subprocess.run(invoked, cwd=base, env=env,
                                           capture_output=True, text=True, timeout=20)
                if editor_result is not None:
                    if ((editor_result.returncode, output(editor_result.stdout, paths),
                         output(editor_result.stderr, paths)) !=
                            (completed.returncode, output(completed.stdout, paths),
                             output(completed.stderr, paths)) or editor_after != snapshot(base)):
                        raise AssertionError(f"Go/Python editor output or exact file bytes differ: {args}")
                if (compare_read_only and step["args"][0] in ("validate", "projects", "config", "guidance")
                        and step.get("root", True)):
                    candidate = subprocess.run(
                        compare_read_only + ["--root", str(root)] + args,
                        cwd=base, env=env, capture_output=True, text=True, timeout=20)
                    actual_result = (candidate.returncode, output(candidate.stdout, paths),
                                     output(candidate.stderr, paths))
                    reference_result = (completed.returncode, output(completed.stdout, paths),
                                        output(completed.stderr, paths))
                    if step["args"][0] == "guidance" and isinstance(reference_result[1], dict):
                        # Approved native difference: findings are lexically ordered,
                        # not filesystem-ordered. Preserve duplicates and require
                        # the candidate's actual order rather than sorting it here.
                        reference_result[1]["errors"] = sorted(reference_result[1]["errors"])
                    if actual_result != reference_result:
                        raise AssertionError(
                            f"{case['id']} {args}: Go={actual_result!r}; Python={reference_result!r}")
                    if snapshot(base) != before:
                        raise AssertionError(f"Read-only command wrote files: {case['id']} {args}")
                after = snapshot(base)
                reports = report_files(root, paths)
                new_reports = [body for name, body in reports.items() if name not in old_reports]
                writes = {name: body for name, body in after.items() if before.get(name) != body}
                # Raw report hashes vary; compare parsed evidence and Markdown separately.
                for name in list(writes):
                    if REPORT.search(name):
                        path = base / name
                        if path.suffix == ".json":
                            del writes[name]
                        else:
                            body = path.read_text()
                            body = re.sub(r"Recorded: [^\n]+", "Recorded: $TIME", body)
                            writes[name] = {"text": body}
                records.append({
                    "exit": completed.returncode,
                    "stdout": output(completed.stdout, paths),
                    "stderr": output(completed.stderr, paths),
                    "writes": normalize(writes, paths),
                    "removed": sorted(set(before) - set(after)),
                    "preserved": sorted(name for name in before if before[name] == after.get(name)
                                        and not REPORT.search(name)),
                    "reports": new_reports})
            results[case["id"]] = records
    return results


def test_map(source):
    owners = {"test_collections.py": "scaffolding", "test_layout.py": "scaffolding",
              "test_services.py": "execution", "test_wsup.py": "cli"}
    overrides = {
        "configuration": "configuration", "bad_configuration": "configuration",
        "report_containment": "configuration", "unknown_registry": "registry",
        "dependency": "registry", "escape_and_symlink": "registry",
        "guidance": "guidance", "ai_kit": "guidance",
        "reports_omit": "reporting", "project_scaffold": "scaffolding",
        "generated": "scaffolding", "update": "scaffolding",
        "legacy": "scaffolding", "manifest": "scaffolding",
        "independent_project": "scaffolding", "gitignore": "scaffolding"}
    result = []
    for path in sorted((source / "tests").glob("test_*.py")):
        if path.name not in owners:
            continue
        tree = ast.parse(path.read_text())
        for cls in (node for node in tree.body if isinstance(node, ast.ClassDef)):
            for method in (node for node in cls.body if isinstance(node, ast.FunctionDef)
                           and node.name.startswith("test_")):
                short = method.name[5:]
                owner = next((package for token, package in overrides.items()
                              if short.startswith(token)), owners[path.name])
                result.append({
                    "python": f"{path.name}:{cls.name}.{method.name}",
                    "behavior": short.replace("_", " "),
                    "go_package": f"internal/{owner}",
                    "go_test": "Test" + "".join(word.capitalize() for word in short.split("_")),
                    "status": "pending"})
    if len(result) != 65:
        raise ValueError(f"Expected 65 baseline tests, found {len(result)}")
    return result


def capture():
    with tempfile.TemporaryDirectory(prefix="wsup-baseline-source-") as temporary:
        source = Path(temporary) / "source"
        source.mkdir()
        archive(BASELINE, source)
        results = replay([sys.executable, "-B", str(source / "src/wsup.py")])
        (DATA / "expected.json").write_text(json.dumps(results, indent=2) + "\n")
        (DATA / "test-map.json").write_text(json.dumps(test_map(source), indent=2) + "\n")
        histories = {}
        for name, commit, entry in (
                ("v0.1.0", "v0.1.0^{commit}", "tools/wsup.py"),
                ("python-baseline", BASELINE, "src/wsup.py")):
            target = Path(temporary) / name
            target.mkdir()
            archive(commit, target)
            destination = Path(temporary) / (name + "-workspace")
            subprocess.run([sys.executable, "-B", str(target / entry),
                            "init", str(destination)], check=True, capture_output=True)
            histories[name] = {
                "commit": subprocess.check_output(["git", "rev-parse", commit],
                                                  cwd=SOURCE, text=True).strip(),
                "files": snapshot(destination)}
        (DATA / "generated-workspaces.json").write_text(json.dumps(histories, indent=2) + "\n")


def verify_archives():
    with tempfile.TemporaryDirectory(prefix="wsup-baseline-replay-") as temporary:
        base = Path(temporary)
        source = base / "source"
        source.mkdir()
        archive(BASELINE, source)
        actual = replay([sys.executable, "-B", str(source / "src/wsup.py")])
        if actual != json.loads((DATA / "expected.json").read_text()):
            raise AssertionError("Pinned archive command fixtures differ")
        if test_map(source) != json.loads((DATA / "test-map.json").read_text()):
            raise AssertionError("Pinned archive test inventory differs")
        histories = json.loads((DATA / "generated-workspaces.json").read_text())
        for name, record in histories.items():
            target = base / name
            target.mkdir()
            archive(record["commit"], target)
            entry = "tools/wsup.py" if name == "v0.1.0" else "src/wsup.py"
            destination = base / (name + "-workspace")
            subprocess.run([sys.executable, "-B", str(target / entry),
                            "init", str(destination)], check=True, capture_output=True)
            if snapshot(destination) != record["files"]:
                raise AssertionError(f"{name} generated workspace bytes differ")
            verify_upgrade(source, destination)
        verify_safety(source)
        print("Pinned command fixtures and historical inventories match; both real "
              "workspace upgrades, 65 original tests, and supplemental safety probes pass.")


def checked(command, cwd, expected_code=0):
    env = {key: value for key, value in os.environ.items()
           if not key.startswith("WSUP_")}
    result = subprocess.run(command, cwd=cwd, env=env,
                            capture_output=True, text=True, timeout=120)
    if result.returncode != expected_code:
        raise AssertionError(f"{command}\n{result.stdout}\n{result.stderr}")
    return result.stdout, result.stderr


def verify_upgrade(source, destination):
    registry = b'{"version":1,"name":"Historical owner","projects":[]}\n'
    preserved = {
        "workspace.json": registry,
        "README.md": b"Owner's workspace map\n",
        "workbench/.local/retained.txt": b"Keep local material\x00\n",
        "apps/real-project/README.md": b"Keep project\n"}
    historical_report = b'{"recorded_at":"2020-01-01","path":"old/location"}\n'
    legacy = (destination / "tools/wsup.py").exists()
    report_path = (".work/quality/" if legacy else ".wsup/reports/") + "historical.json"
    for name, body in {**preserved, report_path: historical_report}.items():
        target = destination / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(body)
    command = [sys.executable, "-B", str(source / "src/wsup.py"),
               "--root", str(destination), "update"]
    before = snapshot(destination)
    stdout, _ = checked(command, source, expected_code=1)
    preview = json.loads(stdout)
    if snapshot(destination) != before or preview["conflicts"] != ["README.md"]:
        raise AssertionError(f"Historical preview lost data or unexpected conflicts: {preview['conflicts']}")
    # A rejected apply must not write any part of the plan.
    failed = subprocess.run(command + ["--apply"], cwd=source,
                            capture_output=True, text=True, timeout=20)
    if failed.returncode != 2 or snapshot(destination) != before:
        raise AssertionError("Conflicting historical apply did not preserve all files")
    stdout, _ = checked(command + ["--keep", "README.md"], source)
    if json.loads(stdout)["conflicts"] or snapshot(destination) != before:
        raise AssertionError("Keep preview must be read-only and conflict-free")
    checked(command + ["--keep", "README.md", "--apply"], source)
    for name, body in preserved.items():
        if (destination / name).read_bytes() != body:
            raise AssertionError(f"Historical migration changed {name}")
    if (destination / ".wsup/reports/historical.json").read_bytes() != historical_report:
        raise AssertionError("Historical report bytes changed")
    if legacy and (destination / ".work/quality/historical.json").exists():
        raise AssertionError("Historical report source was not retired")
    cli = destination / ".wsup/wsup.py"
    for action in ("validate", "guidance"):
        checked([sys.executable, "-B", str(cli), action], destination)
    stdout, _ = checked(command + ["--keep", "README.md"], source)
    settled = json.loads(stdout)
    if settled["conflicts"] or any(item["action"] != "keep-customization"
                                   for item in settled["actions"]):
        raise AssertionError(f"Historical migration did not settle: {settled}")
    child = destination.parent / (destination.name + "-child")
    checked([sys.executable, "-B", str(cli), "init", str(child)], destination)
    checked([sys.executable, "-B", str(child / ".wsup/wsup.py"), "guidance"], child)


def verify_safety(source):
    runner = (
        "import json,sys,unittest\n"
        "suite=unittest.defaultTestLoader.discover('tests')\n"
        "result=unittest.TextTestRunner(verbosity=1).run(suite)\n"
        "print(json.dumps({'run':result.testsRun,'skipped':len(result.skipped)}))\n"
        "sys.exit(0 if result.wasSuccessful() and not result.skipped else 1)\n")
    stdout, stderr = checked([sys.executable, "-B", "-c", runner], source)
    if json.loads(stdout) != {"run": 65, "skipped": 0}:
        raise AssertionError(f"Expected all 65 original tests without skips: {stdout}")
    print(stderr.strip())
    stdout, stderr = checked(
        [sys.executable, "-B", str(SOURCE / "tests/test_migration_safety.py"),
         "--source", str(source)], source)
    print((stdout + stderr).strip())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--capture", action="store_true",
                        help="Explicitly regenerate expectations from pinned Git archives")
    parser.add_argument("--command", nargs=argparse.REMAINDER,
                        help="CLI argv prefix, e.g. --command /path/to/wsup")
    parser.add_argument("--verify-archives", action="store_true",
                        help="Replay pinned archives and both historical generation inventories")
    parser.add_argument("--compare-read-only", nargs=argparse.REMAINDER,
                        help="Compare ported inspection/editor commands while Python drives setup")
    args = parser.parse_args()
    if args.verify_archives:
        if args.capture or args.command or args.compare_read_only:
            parser.error("--verify-archives cannot use --capture or --command")
        verify_archives()
        return
    if args.capture:
        if args.command or args.compare_read_only:
            parser.error("--capture cannot use --command")
        capture()
        print("Captured immutable Python command, test-map, and historical inventories.")
    else:
        command = args.command or [sys.executable, "-B", str(SOURCE / "src/wsup.py")]
        expected = json.loads((DATA / "expected.json").read_text())
        actual = replay(command, args.compare_read_only)
        if actual != expected:
            for case in actual:
                for index, (want, got) in enumerate(zip(expected[case], actual[case])):
                    if want != got:
                        raise AssertionError(f"{case} step {index + 1} differs: "
                                             f"{[key for key in want if want[key] != got[key]]}")
            raise AssertionError("Fixture inventory differs")
        print(f"Passed {sum(map(len, actual.values()))} command steps in {len(actual)} scenarios.")


if __name__ == "__main__":
    main()
