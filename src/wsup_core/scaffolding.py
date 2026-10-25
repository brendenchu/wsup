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
COLLECTIONS = ("archive", "artifacts", "journal", "learning", "private", "reference", "research")
FORMER_DEFAULT_FILES = {
    "journal/README.md", "journal/TEMPLATE.md", "learning/README.md",
    "reference/README.md", "research/README.md",
}
DEFAULT_HOME_FILES = {
    "apps/.gitkeep", "infrastructure/.gitkeep", "packages/README.md",
    "tools/README.md", "workbench/README.md",
}

def omitted_home_files(state):
    names = state.get("omitted_home_files", [])
    if (not isinstance(names, list) or any(
            not isinstance(name, str) or name not in DEFAULT_HOME_FILES for name in names)):
        raise ValueError("omitted_home_files must list default workspace home scaffold files")
    if names and state["template"] != "workspace":
        raise ValueError("Home scaffold omissions require a workspace template")
    return sorted(set(names))

def selected_collections(names):
    if not isinstance(names, (list, tuple)) or any(
            not isinstance(name, str) or name not in COLLECTIONS for name in names):
        raise ValueError("Collections must be selected from: " + ", ".join(COLLECTIONS))
    return sorted(set(names))

def collection_files(names):
    files = {}
    for name in selected_collections(names):
        for relative, body in read_tree(catalogue() / "collections" / name).items():
            files[name + "/" + relative] = body
    return files

def catalogue():
    return Path(__file__).resolve().parent.parent / "templates"

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
    return read_tree(base / "workspace")

def bundle(kind, collections=()):
    collections = selected_collections(collections)
    if collections and kind != "workspace":
        raise ValueError("Optional collections require the workspace template")
    base = catalogue()
    output = workspace_template(base) if kind == "workspace" else read_tree(base / kind)
    if kind == "workspace":
        output.update(collection_files(collections))
        tools = Path(__file__).resolve().parent.parent
        output[".wsup/wsup.py"] = (tools / "wsup.py").read_bytes()
        for name, body in read_tree(tools / "wsup_core").items():
            output[".wsup/wsup_core/" + name] = body
        # Carry template sources so generated copies can initialise another workspace.
        for family in ("workspace", "project", "collections"):
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

def state_bytes(kind, files, collections=()):
    state = {"version": 1, "template_version": VERSION, "template": kind,
             "files": {name: digest(body) for name, body in files.items()}}
    if collections:
        state["collections"] = selected_collections(collections)
    return (json.dumps(state, indent=2) + "\n").encode()

def initialise(destination, kind="workspace", collections=()):
    if destination.exists() or destination.is_symlink():
        raise ValueError("Destination must not exist; existing work is never overwritten")
    destination = destination.absolute()
    files = bundle(kind, collections)
    destination.parent.mkdir(parents=True, exist_ok=True)
    temporary = Path(tempfile.mkdtemp(prefix=".wsup-init-", dir=destination.parent))
    try:
        for name, body in files.items():
            atomic_write(temporary / name, body)
        atomic_write(temporary / STATE, state_bytes(kind, files, collections))
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

def template_target(root, name):
    target = relative_target(root, name)
    if any(parent.is_symlink() for parent in [target, *target.parents] if parent.is_relative_to(root)):
        raise ValueError("Template operation refuses symlink destinations")
    if any(parent.exists() and not parent.is_dir() for parent in target.parents if parent.is_relative_to(root)):
        raise ValueError(f"Template parent is not a directory: {name}")
    return target

def add_collections(root, names=(), apply=False):
    names = selected_collections(names)
    if not names:
        if apply:
            raise ValueError("Select a collection with --add before using --apply")
        return {"available": list(COLLECTIONS), "actions": [], "conflicts": [], "applied": False}
    state_path = template_target(root, STATE)
    before = state_path.read_bytes()
    state = read_json(state_path)
    if (state.get("version") != 1 or state.get("template") != "workspace"
            or not isinstance(state.get("files"), dict)):
        raise ValueError("Adding collections requires a managed workspace")
    enabled = selected_collections(state.get("collections", []))
    files = collection_files(names)
    actions, conflicts = [], []
    for name in sorted(files):
        target = template_target(root, name)
        if name in state["files"]:
            # Previously default-managed scaffolds can be explicitly selected too.
            if (name.split("/", 1)[0] not in enabled and name not in FORMER_DEFAULT_FILES
                    or not target.is_file()):
                conflicts.append(name)
        elif target.exists():
            conflicts.append(name)
        else:
            actions.append({"path": name, "action": "write"})
    result = {"selected": names, "actions": actions, "conflicts": conflicts, "applied": False}
    if apply:
        if conflicts:
            raise ValueError("Collection conflicts require manual resolution; no files updated: "
                             + ", ".join(conflicts))
        if state_path.read_bytes() != before:
            raise ValueError("Template state changed during collection planning")
        for item in actions:
            target = template_target(root, item["path"])
            if target.exists():
                raise ValueError("Collection destination changed during planning: " + item["path"])
        for item in actions:
            target = template_target(root, item["path"])
            target.parent.mkdir(parents=True, exist_ok=True)
            # Exclusive creation never replaces a file created after preflight.
            with target.open("xb") as stream:
                stream.write(files[item["path"]])
            state["files"][item["path"]] = digest(files[item["path"]])
        state["collections"] = sorted(set(enabled) | set(names))
        atomic_write(state_path, (json.dumps(state, indent=2) + "\n").encode())
        result["applied"] = True
    return result

def migrated_name(name):
    for family in ("workspace", "project", "collections"):
        for base in ("tools/wsup_templates/", ".wsup/templates/"):
            prefix = base + family + "/"
            if name.startswith(prefix):
                if family == "workspace" and name[len(prefix):] in FORMER_DEFAULT_FILES:
                    return None
                inner = migrated_name(name[len(prefix):])
                return None if inner is None else ".wsup/templates/" + family + "/" + inner
    if name.startswith("templates/project/"):
        return None
    for previous, current in (
        (".ai/rules/", "guidance/rules/"),
        (".ai/skills/", ".agents/skills/"),
        ("tools/wsup_core/", ".wsup/wsup_core/"),
    ):
        if name.startswith(previous):
            return current + name[len(previous):]
    return ".wsup/wsup.py" if name == "tools/wsup.py" else name

def update(root, apply=False, keep=()):
    state_path = relative_target(root, STATE)
    state_before = state_path.read_bytes()
    state = read_json(state_path)
    if state.get("version") != 1 or state.get("template") not in ("workspace", "project") or not isinstance(state.get("files"), dict):
        raise ValueError("Unsupported template state")
    collections = selected_collections(state.get("collections", []))
    files = bundle(state["template"], collections)
    omitted = omitted_home_files(state)
    files = {name: body for name, body in files.items() if name not in omitted}
    old = state["files"]
    actions, conflicts, observed = [], [], {}
    kept = set(keep)

    def target_for(name):
        return template_target(root, name)

    def observe(name):
        target = target_for(name)
        if target.exists() and not target.is_file():
            conflicts.append(name)
            return None
        current = digest(target.read_bytes()) if target.is_file() else None
        observed[name] = current
        return current

    target_for(STATE)
    for name in kept:
        target_for(name)
        if (name not in old or
                migrated_name(name) != name or
                not (root / name).is_file()):
            raise ValueError("--keep requires an existing same-path managed file: " + name)

    # Snapshot only retired/moved tool trees, never independent projects under tools/.
    legacy_trees = {}
    for prefix in (".ai/rules/", ".ai/skills/",
                   ".wsup/templates/workspace/.ai/skills/",
                   ".wsup/templates/workspace/.ai/rules/",
                   ".wsup/templates/project/.ai/rules/", "tools/wsup_core/",
                   "tools/wsup_templates/", "templates/project/"):
        directory = target_for(prefix)
        tree = read_tree(directory) if directory.exists() else {}
        legacy_trees[prefix] = tree
        for relative in tree:
            name = prefix + relative
            if name not in old:
                conflicts.append(name)

    handled = set()
    destinations = set()
    for name in sorted(old):
        destination = migrated_name(name)
        if destination == name:
            continue
        handled.add(name)
        current = observe(name)
        if name in kept:
            actions.append({"path": name, "action": "keep-customization"})
            continue
        if current is None or current != old[name]:
            conflicts.append(name)
            continue
        if destination is None:
            actions.append({"path": name, "action": "retire"})
        elif destination not in files:
            conflicts.append(name)
        elif destination in destinations or destination in old or target_for(destination).exists():
            conflicts.append(destination)
        else:
            observe(destination)
            destinations.add(destination)
            actions.append({"path": destination, "action": "relocate", "from": name})

    for name in sorted((set(files) | set(old)) - handled - destinations):
        current = observe(name)
        if name == "workspace.json":
            continue
        if name in kept:
            actions.append({"path": name, "action": "keep-customization"})
        elif name not in files:
            actions.append({"path": name, "action": "retain-orphan"})
        elif current == digest(files[name]):
            continue
        elif current == old.get(name):
            actions.append({"path": name, "action": "write"})
        else:
            conflicts.append(name)

    # Reports are evidence, not templates: relocate exact bytes without rewriting provenance.
    report_prefix = ".work/quality/"
    reports_dir = target_for(report_prefix)
    reports = read_tree(reports_dir) if reports_dir.exists() else {}
    for relative in reports:
        name = report_prefix + relative
        destination = ".wsup/reports/" + relative
        observe(name)
        observe(destination)
        if target_for(destination).exists():
            conflicts.append(destination)
        else:
            actions.append({"path": destination, "action": "move-report", "from": name})

    # Old explicit report settings need an owner-reviewed change, not a silent fallback.
    for filename in ("wsup.json", "wsup.local.json"):
        path = target_for(filename)
        if path.is_file():
            observe(filename)
            settings = read_json(path).get("settings", {})
            if isinstance(settings, dict) and str(settings.get("report_dir", "")).startswith(".work"):
                conflicts.append(filename)

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
                raise ValueError("Legacy files changed during update planning")
        if (read_tree(reports_dir) if reports_dir.exists() else {}) != reports:
            raise ValueError("Reports changed during update planning")
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
            elif item["action"] == "move-report":
                atomic_write(root / item["path"], reports[item["from"][len(report_prefix):]])
        for item in actions:
            if item["action"] in ("relocate", "retire", "move-report"):
                name = item.get("from", item["path"])
                source = target_for(name)
                if not source.is_file() or digest(source.read_bytes()) != observed[name]:
                    raise ValueError("Source changed during migration; new files may already be written")
                source.unlink()
        # Only remove empty ancestors of exact sources removed above.
        for item in actions:
            if item["action"] not in ("relocate", "retire", "move-report"):
                continue
            directory = (root / item.get("from", item["path"])).parent
            while directory != root and directory.is_dir() and not any(directory.iterdir()):
                directory.rmdir()
                directory = directory.parent
        tracked = {name: body for name, body in files.items() if name != "workspace.json"}
        next_state = json.loads(state_bytes(state["template"], tracked, collections))
        if omitted:
            next_state["omitted_home_files"] = omitted
        if "workspace.json" in old:
            next_state["files"]["workspace.json"] = old["workspace.json"]
        for name in kept & set(files):
            next_state["files"][name] = old[name]
        atomic_write(state_path, (json.dumps(next_state, indent=2) + "\n").encode())
        result["applied"] = True
    return result
