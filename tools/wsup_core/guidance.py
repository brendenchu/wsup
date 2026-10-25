"""Inspect portable instructions without requiring an agent or loading remote text."""
from pathlib import Path
import re
from .registry import relative_target

def inspect(root):
    errors, files, skills = [], [], []
    for name in ("AI.md", "guidance/rules/index.md"):
        if not (root / name).is_file():
            errors.append(f"Missing {name}")
    candidates = [root / "AI.md", root / "AGENTS.md"]
    ai = root / "guidance"
    if ai.exists():
        candidates.extend(ai.rglob("*.md"))
    shared_skills = root / ".agents/skills"
    if shared_skills.exists():
        candidates.extend(shared_skills.rglob("*.md"))
    legacy_skills = root / ".ai/skills"
    if (root / ".ai/rules").is_dir():
        errors.append("Legacy .ai/rules require an explicit layout migration to guidance/rules")
    coexist = any(shared_skills.rglob("SKILL.md")) and any(legacy_skills.rglob("SKILL.md"))
    if coexist:
        errors.append("Legacy skills in .ai/skills coexist with .agents/skills; resolve migration manually")
    for path in candidates:
        if not path.is_file():
            continue
        if not path.resolve().is_relative_to(root.resolve()):
            errors.append("Guidance symlink escapes owner")
            continue
        rel = path.relative_to(root).as_posix()
        files.append(rel)
        text = path.read_text(encoding="utf-8")
        for link in re.findall(r"\]\(([^)]+)\)", text):
            if re.match(r"[a-zA-Z][a-zA-Z0-9+.-]*:", link) or link.startswith("#"):
                continue
            target = (path.parent / link.split("#", 1)[0]).resolve()
            if not target.is_relative_to(root.resolve()) or not target.is_file():
                errors.append(f"{rel}: unresolved/nonlocal reference")
        if path.name == "SKILL.md":
            try:
                if not text.startswith("---\n"):
                    raise ValueError()
                header, body = text[4:].split("\n---\n", 1)
                fields = dict(line.split(": ", 1) for line in header.splitlines())
                if fields.get("name") != path.parent.name or not re.fullmatch(r"[a-z0-9-]{1,63}", fields["name"]):
                    raise ValueError()
                if not fields.get("description", "").strip() or not body.strip():
                    raise ValueError()
                if not (coexist and path.is_relative_to(legacy_skills)):
                    skills.append({"name": fields["name"], "path": rel})
            except (ValueError, KeyError):
                errors.append(f"{rel}: expected simple name/description frontmatter and body")
    return {"files": sorted(set(files)), "skills": sorted(skills, key=lambda x: x["name"]), "errors": errors}
