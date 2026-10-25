"""Sanitised local evidence; never persist command arguments, env or tool output."""
from collections import Counter
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import uuid
from . import VERSION

def digest(data):
    return hashlib.sha256(data).hexdigest()

def git_state(root):
    try:
        head = subprocess.run(["git", "rev-parse", "HEAD"], cwd=root, capture_output=True, timeout=2)
        if head.returncode:
            return {"available": False}
        status = subprocess.run(["git", "-c", "core.fsmonitor=false", "status", "--porcelain"], cwd=root,
                                capture_output=True, timeout=2)
        return {"available": True, "head": head.stdout.decode().strip(),
                "dirty": bool(status.stdout) if status.returncode == 0 else None}
    except (OSError, subprocess.TimeoutExpired):
        return {"available": False}

def write_report(root, config, selected, results, config_sources):
    directory = root / config["report_dir"]
    directory.mkdir(parents=True, exist_ok=True)
    now = datetime.now(timezone.utc)
    stem = now.strftime("%Y%m%dT%H%M%S") + "-" + uuid.uuid4().hex[:12]
    report = {"version": 1, "wsup_version": VERSION, "recorded_at": now.isoformat(),
              "runtime": {"python": platform.python_version(), "os": platform.system()},
              "git": git_state(root), "config_sources": config_sources,
              "inputs": {name: digest((root / name).read_bytes()) for name in
                         ("workspace.json", "wsup.json", "wsup.local.json") if (root / name).is_file()},
              "selection": [{"project": p["id"], "check": name, "path": p["path"],
                             "command_sha256": digest(json.dumps(spec["command"]).encode())}
                            for p, name, spec in selected],
              "results": results, "summary": dict(Counter(item["status"] for item in results))}
    json_path = directory / (stem + ".json")
    md_path = directory / (stem + ".md")
    markdown = "# Local verification\n\nRecorded: " + report["recorded_at"] + "\n\n"
    markdown += "\n".join(f"- {x['project']}:{x['check']}: {x['status']}" for x in results) + "\n"
    for path, content in [(json_path, json.dumps(report, indent=2) + "\n"), (md_path, markdown)]:
        descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(descriptor, "w", encoding="utf-8", newline="\n") as stream:
            stream.write(content)
    return json_path
