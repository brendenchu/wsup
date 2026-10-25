"""One CLI over registry, settings, execution, guidance and scaffold services."""
import argparse
import json
from pathlib import Path
import sys
from . import VERSION
from .configuration import load_config
from .execution import run
from .guidance import inspect
from .registry import load, select, relative_target
from .reporting import write_report
from .scaffolding import COLLECTIONS, add_collections, atomic_write, initialise, update

def parser():
    p = argparse.ArgumentParser(description="WSUP portable local workspace tools")
    p.add_argument("--version", action="version", version=VERSION)
    p.add_argument("--root", type=Path, default=Path.cwd())
    commands = p.add_subparsers(dest="action", required=True)
    for name in ("validate", "projects", "config"):
        commands.add_parser(name)
    guidance = commands.add_parser("guidance")
    guidance.add_argument("--project")
    editor = commands.add_parser("editor", help="Optional VS Code workspace output")
    editor.add_argument("--output", default="workspace.code-workspace")
    check = commands.add_parser("check")
    check.add_argument("--project", action="append", required=True)
    check.add_argument("--check", action="append", dest="names")
    check.add_argument("--timeout", type=int)
    check.add_argument("--show-output", action="store_true")
    init = commands.add_parser("init")
    init.add_argument("destination", type=Path)
    init.add_argument("--template", choices=("workspace", "project"), default="workspace")
    init.add_argument("--collection", choices=COLLECTIONS, action="append", default=[],
                      help="Include an optional collection; repeat to select more (workspace only)")
    collections = commands.add_parser("collections", help="List or scaffold optional workspace collections")
    collections.add_argument("--add", choices=COLLECTIONS, action="append", default=[])
    collections.add_argument("--apply", action="store_true", help="Apply the preview without overwriting existing files")
    upgrade = commands.add_parser("update")
    upgrade.add_argument("--apply", action="store_true")
    upgrade.add_argument("--keep", action="append", default=[], metavar="PATH",
                         help="Explicitly preserve an existing same-path customization; repeat per file")
    return p

def check(root, projects, args, config, sources):
    selected = select(projects, args.project, args.names)
    # Preflight the report location before executing commands.
    report_dir = relative_target(root, config["report_dir"])
    report_dir.mkdir(parents=True, exist_ok=True)
    results, cancelled = [], False
    for project, name, spec in selected:
        label = {"project": project["id"], "check": name, "mode": spec["mode"]}
        if cancelled or not spec["enabled"]:
            result = {**label, "status": "skipped", "reason": "cancelled" if cancelled else spec["reason"]}
        else:
            print(f"Running {project['id']}:{name}", flush=True)
            try:
                execution = run(spec["command"], root / project["path"],
                                args.timeout or spec["timeout_seconds"] or config["timeout_seconds"], config["output_limit_bytes"])
            except KeyboardInterrupt:
                execution = {"status": "cancelled", "exit_code": None, "stdout": "", "stderr": ""}
            if args.show_output:
                for stream in ("stdout", "stderr"):
                    if execution.get(stream):
                        print(execution[stream], end="" if execution[stream].endswith("\n") else "\n")
            # Tool output may contain secrets; reports intentionally omit it.
            result = {**label, **{k: v for k, v in execution.items() if k not in ("stdout", "stderr")}}
            if result["status"] == "failed" and spec["mode"] == "report" and result["exit_code"] in spec["finding_exit_codes"]:
                result["status"] = "findings"
            cancelled = result["status"] == "cancelled"
        results.append(result)
        print(f"{project['id']}:{name}: {result['status']}")
    path = write_report(root, config, selected, results, sources)
    print(f"Report: {path.relative_to(root).as_posix()}")
    if cancelled:
        return 130
    failed = any(r["status"] in ("error", "timeout", "failed") or
                 (r["status"] == "skipped" and r["mode"] == "required") for r in results)
    return int(failed)

def main(argv=None):
    args = parser().parse_args(argv)
    try:
        root = args.root.resolve()
        if args.action == "init":
            print(f"Created {initialise(args.destination, args.template, args.collection)}")
            return 0
        if args.action == "collections":
            result = add_collections(root, args.add, args.apply)
            print(json.dumps(result, indent=2))
            return int(bool(result["conflicts"]))
        if args.action == "update":
            result = update(root, args.apply, args.keep)
            print(json.dumps(result, indent=2))
            return int(bool(result["conflicts"]))
        if args.action == "guidance" and not args.project:
            result = inspect(root)
            print(json.dumps(result, indent=2))
            return int(bool(result["errors"]))
        data, projects = load(root)
        config, sources = load_config(root, {"timeout_seconds": getattr(args, "timeout", None)})
        if args.action == "validate":
            print(f"Valid: {data['name']} ({len(projects)} projects)")
        elif args.action == "projects":
            print(json.dumps(data["projects"], indent=2))
        elif args.action == "config":
            print(json.dumps({"settings": config, "sources": sources}, indent=2))
        elif args.action == "guidance":
            if args.project not in projects:
                raise ValueError("Unknown project")
            result = inspect(relative_target(root, projects[args.project]["path"]))
            print(json.dumps(result, indent=2))
            return int(bool(result["errors"]))
        elif args.action == "editor":
            output = relative_target(root, args.output)
            if output.parent != root or output.exists() or output.is_symlink():
                raise ValueError("Editor output must be a new file in the workspace root")
            document = {"folders": [{"name": "Workspace", "path": "."}] + [
                {"name": p["id"], "path": p["path"]} for p in projects.values() if p["path"] != "."
            ], "settings": {}}
            # Exclusive creation preserves existing custom editor files.
            with output.open("x", encoding="utf-8", newline="\n") as stream:
                stream.write(json.dumps(document, indent=2) + "\n")
            print(f"Created {output}")
        elif args.action == "check":
            return check(root, projects, args, config, sources)
        return 0
    except (ValueError, OSError, TypeError, KeyError, RecursionError) as error:
        print(f"Error: {error}", file=sys.stderr)
        return 2
    except KeyboardInterrupt:
        return 130
