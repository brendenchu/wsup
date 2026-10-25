"""Validate project ownership and check contracts without running project code."""
import json
from pathlib import Path, PurePosixPath
import re

KINDS = {"app", "package", "tool", "infrastructure", "research", "template"}
ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_.-]*$")

def relative_target(root, value):
    if not isinstance(value, str) or not value or "\\" in value or ":" in value:
        raise ValueError("Use a nonempty relative forward-slash path")
    path = PurePosixPath(value)
    if path.is_absolute() or ".." in path.parts:
        raise ValueError("Paths must stay inside the workspace")
    target = root.joinpath(*path.parts)
    if not target.resolve().is_relative_to(root.resolve()):
        raise ValueError("Path or symlink escapes the workspace")
    return target

def read_json(path):
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"{path.name} must contain an object")
    return value

def check_definition(raw):
    value = {"command": raw} if isinstance(raw, list) else raw
    if not isinstance(value, dict) or set(value) - {"command", "mode", "enabled", "reason", "timeout_seconds", "finding_exit_codes"}:
        raise ValueError("Check must be an argument list or supported check object")
    command = value.get("command")
    if not isinstance(command, list) or not command or any(not isinstance(a, str) or "\0" in a for a in command) or not command[0]:
        raise ValueError("Check command must be a nonempty list of literal string arguments")
    mode = value.get("mode", "required")
    if mode not in ("required", "report") or not isinstance(value.get("enabled", True), bool):
        raise ValueError("Check mode must be required/report and enabled must be boolean")
    if "reason" in value and not isinstance(value["reason"], str):
        raise ValueError("Check reason must be a string")
    if value.get("enabled", True) is False and not value.get("reason", "").strip():
        raise ValueError("Disabled checks need a reason")
    finding_codes = value.get("finding_exit_codes", [1])
    if not isinstance(finding_codes, list) or any(type(code) is not int or not 1 <= code <= 255 for code in finding_codes):
        raise ValueError("finding_exit_codes must contain exit codes 1..255")
    timeout = value.get("timeout_seconds")
    if timeout is not None and (type(timeout) is not int or not 1 <= timeout <= 86400):
        raise ValueError("Check timeout_seconds must be 1..86400")
    return {"command": command, "mode": mode, "enabled": value.get("enabled", True),
            "reason": value.get("reason"), "timeout_seconds": timeout, "finding_exit_codes": finding_codes}

def load(root):
    root = root.resolve()
    data = read_json(root / "workspace.json")
    if set(data) - {"version", "name", "projects"} or type(data.get("version")) is not int or data["version"] != 1:
        raise ValueError("workspace.json requires supported version 1 fields")
    if not isinstance(data.get("name"), str) or not data["name"].strip() or not isinstance(data.get("projects"), list):
        raise ValueError("Registry needs a name and projects list")
    indexed, paths = {}, set()
    for p in data["projects"]:
        if not isinstance(p, dict) or set(p) - {"id", "purpose", "owner", "path", "kind", "repository", "dependencies", "checks"}:
            raise ValueError("Invalid project fields")
        if not isinstance(p.get("id"), str) or not ID.fullmatch(p["id"]) or p["id"] in indexed:
            raise ValueError("Project IDs must be unique letters/digits/dots/underscores/hyphens")
        for field in ("purpose", "owner", "path"):
            if not isinstance(p.get(field), str) or not p[field].strip():
                raise ValueError(f"{p['id']}: missing {field}")
        if p.get("kind") not in KINDS or p.get("repository") not in ("workspace", "independent"):
            raise ValueError(f"{p['id']}: invalid kind/repository")
        target = relative_target(root, p["path"])
        if not target.is_dir() or target.resolve() in paths or not (target / "README.md").is_file():
            raise ValueError(f"{p['id']}: path must be unique/existing and contain README.md")
        paths.add(target.resolve())
        deps = p.get("dependencies", [])
        if not isinstance(deps, list) or any(not isinstance(d, str) for d in deps):
            raise ValueError(f"{p['id']}: dependencies must be project IDs")
        checks = p.get("checks", {})
        if not isinstance(checks, dict):
            raise ValueError(f"{p['id']}: checks must be named definitions")
        for name, raw in checks.items():
            if not ID.fullmatch(name):
                raise ValueError(f"{p['id']}: invalid check name")
            check_definition(raw)
        indexed[p["id"]] = p
    done, active = set(), set()
    def visit(key):
        if key not in indexed:
            raise ValueError(f"Unknown dependency: {key}")
        if key in active:
            raise ValueError(f"Dependency cycle at {key}")
        if key in done:
            return
        active.add(key)
        for dep in indexed[key].get("dependencies", []):
            visit(dep)
        active.remove(key)
        done.add(key)
    for key in indexed:
        visit(key)
    return data, indexed

def select(projects, requested, names=None):
    selected = []
    for key in dict.fromkeys(requested):
        if key not in projects:
            raise ValueError(f"Unknown project: {key}")
        p = projects[key]
        chosen = list(dict.fromkeys(names)) if names else list(p.get("checks", {}))
        if not chosen:
            raise ValueError(f"{key}: no checks configured")
        for name in chosen:
            if name not in p.get("checks", {}):
                raise ValueError(f"{key}: unknown check {name}")
            selected.append((p, name, check_definition(p["checks"][name])))
    return selected
