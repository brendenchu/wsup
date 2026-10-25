"""Versioned templates, standalone copies and non-destructive update planning."""
import json
import os
from pathlib import Path
import shutil
import tempfile
from . import VERSION
from .registry import read_json, relative_target
from .reporting import digest

STATE = ".wsup/template-state.json"

def catalogue():
    tools = Path(__file__).resolve().parent.parent
    return tools / "templates" if tools.name == ".wsup" else tools.parent / "templates"

def read_tree(source):
    if not source.is_dir():
        raise ValueError("Template catalogue is unavailable")
    output = {}
    for p in sorted(source.rglob("*")):
        if p.is_symlink():
            raise ValueError("Templates must not contain symlinks")
        if p.is_file() and "__pycache__" not in p.parts and not p.name.endswith(".pyc"):
            output[p.relative_to(source).as_posix()] = p.read_bytes()
    return output

def workspace_template(base):
    files = read_tree(base / "workspace")
    tools = Path(__file__).resolve().parent.parent
    if tools.name == ".wsup":
        return files
    skills = read_tree(tools.parent / ".agents" / "skills")
    files = {name: body for name, body in files.items()
             if not name.startswith((".ai/skills/", ".agents/skills/"))}
    files.update({".agents/skills/" + name: body for name, body in skills.items()})
    return files

def bundle(kind):
    base = catalogue()
    output = workspace_template(base) if kind == "workspace" else read_tree(base / kind)
    if kind == "workspace":
        tools = Path(__file__).resolve().parent.parent
        output[".wsup/wsup.py"] = (tools / "wsup.py").read_bytes()
        for name, body in read_tree(tools / "wsup_core").items():
            output[".wsup/wsup_core/" + name] = body
        # Carry template sources so generated copies can initialise another workspace.
        for family in ("workspace", "project"):
            files = workspace_template(base) if family == "workspace" else read_tree(base / family)
            for name, body in files.items():
                output[".wsup/templates/" + family + "/" + name] = body
    return output

def atomic_write(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)
    handle, temp = tempfile.mkstemp(prefix=".wsup-", dir=path.parent)
    try:
        with os.fdopen(handle, "wb") as stream:
            stream.write(content)
        os.replace(temp, path)
    finally:
        if Path(temp).exists():
            Path(temp).unlink()

def state_bytes(kind, files):
    return (json.dumps({"version": 1, "template_version": VERSION, "template": kind,
        "files": {name: digest(body) for name, body in files.items()}}, indent=2) + "\n").encode()

def initialise(destination, kind="workspace"):
    if destination.exists() or destination.is_symlink():
        raise ValueError("Destination must not exist; existing work is never overwritten")
    destination = destination.absolute()
    files = bundle(kind)
    destination.parent.mkdir(parents=True, exist_ok=True)
    temporary = Path(tempfile.mkdtemp(prefix=".wsup-init-", dir=destination.parent))
    try:
        for name, body in files.items():
            atomic_write(temporary / name, body)
        atomic_write(temporary / STATE, state_bytes(kind, files))
        # Refuse a destination created concurrently; never replace an existing directory.
        destination.mkdir()
        try:
            for child in temporary.iterdir():
                shutil.move(str(child), destination / child.name)
        except BaseException:
            shutil.rmtree(destination)
            raise
    finally:
        shutil.rmtree(temporary, ignore_errors=True)
    return destination

def update(root, apply=False):
    state_path = relative_target(root, STATE)
    state_before = state_path.read_bytes()
    state = read_json(state_path)
    if state.get("version") != 1 or state.get("template") not in ("workspace", "project") or not isinstance(state.get("files"), dict):
        raise ValueError("Unsupported template state")
    files = bundle(state["template"])
    old = state["files"]
    actions, conflicts = [], []

    def target_for(name):
        target = relative_target(root, name)
        if any(parent.is_symlink() for parent in [target, *target.parents] if parent.is_relative_to(root)):
            raise ValueError("Template update refuses symlink destinations")
        if any(parent.exists() and not parent.is_dir() for parent in target.parents if parent.is_relative_to(root)):
            raise ValueError(f"Template parent is not a directory: {name}")
        return target

    observed, legacy_names, relocated = {}, set(), set()
    legacy_trees = {}
    if state["template"] == "workspace":
        for prefix in ("", "tools/wsup_templates/workspace/"):
            legacy_prefix = prefix + ".ai/skills/"
            directory = target_for(legacy_prefix)
            tree = read_tree(directory) if directory.exists() else {}
            legacy_trees[legacy_prefix] = tree
            names = {name for name in old if name.startswith(legacy_prefix)}
            names.update(legacy_prefix + name for name in tree)
            legacy_names.update(names)
            for name in sorted(names):
                destination = prefix + ".agents/skills/" + name[len(legacy_prefix):]
                source = target_for(name)
                target = target_for(destination)
                current = digest(source.read_bytes()) if source.is_file() else None
                if name not in old or current is None or current != old[name] or destination not in files:
                    conflicts.append(name)
                elif destination in old or target.exists():
                    conflicts.append(destination)
                else:
                    observed[name], observed[destination] = current, None
                    relocated.add(destination)
                    actions.append({"path": destination, "action": "relocate", "from": name})

    for name in sorted((set(files) | set(old)) - legacy_names - relocated):
        target = target_for(name)
        if target.exists() and not target.is_file():
            conflicts.append(name)
            continue
        current = digest(target.read_bytes()) if target.is_file() else None
        observed[name] = current
        desired = digest(files[name]) if name in files else None
        if name == "workspace.json":
            # Registry belongs to the user once generated, even if still unchanged.
            continue
        if current == desired:
            continue
        if name not in files:
            actions.append({"path": name, "action": "retain-orphan"})
        elif current == old.get(name):
            actions.append({"path": name, "action": "write"})
        else:
            conflicts.append(name)
    result = {"from_version": state.get("template_version"), "to_version": VERSION,
              "actions": sorted(actions, key=lambda x: x["path"]), "conflicts": sorted(set(conflicts)), "applied": False}
    if apply:
        if conflicts:
            raise ValueError("Template conflicts require manual resolution; no files updated: " + ", ".join(sorted(conflicts)))
        # Recheck the preflight inputs immediately before the first write.
        if state_path.read_bytes() != state_before:
            raise ValueError("Template state changed during update planning")
        for prefix, tree in legacy_trees.items():
            directory = target_for(prefix)
            if (read_tree(directory) if directory.exists() else {}) != tree:
                raise ValueError("Legacy skills changed during update planning")
        for name, expected in observed.items():
            target = target_for(name)
            if target.exists() and not target.is_file():
                raise ValueError("Template file changed during update planning")
            current = digest(target.read_bytes()) if target.is_file() else None
            if current != expected:
                raise ValueError("Template file changed during update planning")
        for item in actions:
            if item["action"] in ("write", "relocate"):
                atomic_write(root / item["path"], files[item["path"]])
        for item in actions:
            if item["action"] == "relocate":
                source = target_for(item["from"])
                if not source.is_file() or digest(source.read_bytes()) != observed[item["from"]]:
                    raise ValueError("Legacy skill changed during migration; new files may already be written")
                source.unlink()
        tracked = {name: body for name, body in files.items() if name != "workspace.json"}
        next_state = json.loads(state_bytes(state["template"], tracked))
        if "workspace.json" in old:
            next_state["files"]["workspace.json"] = old["workspace.json"]
        atomic_write(state_path, (json.dumps(next_state, indent=2) + "\n").encode())
        result["applied"] = True
    return result
