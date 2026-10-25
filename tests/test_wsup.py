import contextlib
import importlib.util
import io
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

SOURCE = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("wsup", SOURCE / "src/wsup.py")
wsup = importlib.util.module_from_spec(spec)
spec.loader.exec_module(wsup)

class WsupTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        (self.root / "app").mkdir()
        (self.root / "app/README.md").write_text("Local instructions")
        self.data = {"version": 1, "name": "Synthetic", "projects": [{
            "id": "app", "path": "app", "kind": "app", "repository": "independent",
            "purpose": "Fixture", "owner": "Test", "checks": {}}]}
        self.save()
    def save(self):
        (self.root / "workspace.json").write_text(json.dumps(self.data))
    def run_cli(self, *args):
        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
            return wsup.main(["--root", str(self.root), *args])
    def test_editor_is_relative_and_preserves_existing_output(self):
        self.assertEqual(0, self.run_cli("editor"))
        output = self.root / "workspace.code-workspace"
        self.assertEqual("app", json.loads(output.read_text())["folders"][1]["path"])
        original = output.read_bytes()
        self.assertEqual(2, self.run_cli("editor"))
        self.assertEqual(original, output.read_bytes())
    def test_dependency_cycle_and_unknown_dependency(self):
        for dependency in ["app", "missing"]:
            self.data["projects"][0]["dependencies"] = [dependency]
            self.save()
            self.assertEqual(2, self.run_cli("validate"))
    def test_escape_and_symlink_escape_rejected(self):
        self.data["projects"][0]["path"] = "../"
        self.save()
        self.assertEqual(2, self.run_cli("validate"))
        try:
            (self.root / "outside").symlink_to(self.root.parent, target_is_directory=True)
        except (OSError, NotImplementedError):
            self.skipTest("Symlink creation unavailable on this OS or account")
        self.data["projects"][0]["path"] = "outside"
        self.save()
        self.assertEqual(2, self.run_cli("validate"))
    def test_checks_continue_and_arguments_are_literal(self):
        marker = self.root / "app/result.json"
        self.data["projects"][0]["checks"] = {
            "fail": [sys.executable, "-c", "raise SystemExit(7)"],
            "record": ["{python}", "-c", "import sys,json;open('result.json','w').write(json.dumps(sys.argv[1:]))", "$(touch unsafe)", "", "space value"]}
        self.save()
        self.assertEqual(1, self.run_cli("check", "--project", "app"))
        self.assertEqual(["$(touch unsafe)", "", "space value"], json.loads(marker.read_text()))
        self.assertFalse((self.root / "app/unsafe").exists())
    def test_unknown_check_validated_before_any_execution(self):
        self.data["projects"][0]["checks"] = {"ok": [sys.executable, "-c", "open('unexpected','w').close()"]}
        self.save()
        self.assertEqual(2, self.run_cli("check", "--project", "app", "--check", "ok", "--check", "missing"))
        self.assertFalse((self.root / "app/unexpected").exists())
    def test_ai_kit_references_and_skill_structure(self):
        import re
        kit = SOURCE / "src/templates/workspace"
        for template in (kit, SOURCE / "src/templates/project"):
            self.assertEqual(
                (SOURCE / "LICENSE").read_bytes(),
                (template / "WSUP-LICENSE").read_bytes())
        self.assertEqual(
            (SOURCE / "docs/workspace-model.md").read_bytes(),
            (kit / "docs/workspace-model.md").read_bytes())
        rules = SOURCE / "guidance/rules"
        bundled_rules = kit / "guidance/rules"
        self.assertEqual(
            {path.name for path in rules.glob("*.md")},
            {path.name for path in bundled_rules.glob("*.md")})
        for rule in rules.glob("*.md"):
            self.assertEqual(rule.read_bytes(), (bundled_rules / rule.name).read_bytes(),
                             rule.name)
        self.assertNotIn(".wsup/templates/", (rules / "index.md").read_text())
        self.assertIn(
            "routes to local rules in `guidance/rules/`",
            (SOURCE / "AI.md").read_text())
        for guide in kit.rglob("*.md"):
            for target in re.findall(r"\]\(([^)]+)\)", guide.read_text()):
                linked = (guide.parent / target.split("#", 1)[0]).resolve()
                self.assertTrue(linked.is_relative_to(kit.resolve()), target)
                self.assertTrue(linked.is_file(), target)
        skills = list((kit / ".agents/skills").glob("*/SKILL.md"))
        canonical_skills = SOURCE / ".agents/skills"
        source_skills = list(canonical_skills.glob("*/SKILL.md"))
        self.assertEqual({
            "project-onboarding", "registry-checks", "template-lifecycle",
            "ai-guidance-maintenance", "repository-change", "diagnose-issue",
            "review-change", "plan-work", "workspace-check", "infer-conventions",
            "recap-work", "commit-message",
        }, {path.parent.name for path in source_skills})
        self.assertEqual(
            {path.relative_to(canonical_skills) for path in source_skills},
            {path.relative_to(kit / ".agents/skills") for path in skills})
        for skill in skills:
            source = canonical_skills / skill.relative_to(kit / ".agents/skills")
            self.assertEqual(source.read_bytes(), skill.read_bytes())
            route = f"](../../.agents/skills/{skill.parent.name}/SKILL.md)"
            for index in (SOURCE / "guidance/rules/index.md", kit / "guidance/rules/index.md"):
                self.assertIn(route, index.read_text())
            content = skill.read_text()
            self.assertTrue(content.startswith("---\n"))
            header, body = content[4:].split("\n---\n", 1)
            fields = dict(line.split(": ", 1) for line in header.splitlines())
            self.assertEqual(skill.parent.name, fields["name"])
            self.assertRegex(fields["name"], r"^[a-z0-9-]+$")
            self.assertTrue(fields["description"].strip())
            self.assertTrue(body.strip())
        for guide in [*(SOURCE / "guidance").rglob("*.md"), *(SOURCE / ".agents").rglob("*.md")]:
            for target in re.findall(r"\]\(([^)]+)\)", guide.read_text()):
                linked = (guide.parent / target.split("#", 1)[0]).resolve()
                self.assertTrue(linked.is_relative_to(SOURCE.resolve()), target)
                self.assertTrue(linked.is_file(), target)

    def test_gitignore_baselines_match_and_preserve_trackable_content(self):
        baseline = (SOURCE / "src/templates/project/.gitignore").read_text()
        for path in (SOURCE / ".gitignore", SOURCE / "src/templates/workspace/.gitignore"):
            self.assertEqual(baseline, path.read_text().split("\n\n", 1)[0] + "\n")
        self.assertEqual(
            (SOURCE / ".gitignore").read_bytes(),
            (SOURCE / "src/templates/workspace/.gitignore").read_bytes())
        ignored = [
            "workbench/.local/note", "apps/example/workbench/.local/note",
            ".work/note", "apps/example/.work/note", ".wsup/reports/check.json",
            "wsup.local.json", ".env", ".env.local", "apps/example/.env",
            "__pycache__/module.pyc", "tools/__pycache__/module.pyc",
            ".DS_Store", "apps/example/.DS_Store",
        ]
        trackable = [
            ".env.example", "apps/example/.env.example", ".wsup/template-state.json",
            ".wsup/wsup.py", "workspace.json", "workbench/draft.md",
            "apps/example/README.md", "private/README.md",
        ]
        for kind in ("workspace", "project"):
            with self.subTest(kind=kind):
                root = self.root / kind
                self.assertEqual(0, self.run_cli("init", str(root), "--template", kind))
                subprocess.run(["git", "init", "--quiet", str(root)], check=True, capture_output=True)
                result = subprocess.run(
                    ["git", "-c", "core.excludesFile=/dev/null", "check-ignore",
                     "--no-index", "--stdin", "-z"],
                    cwd=root, input="\0".join(ignored + trackable) + "\0",
                    text=True, capture_output=True)
                self.assertEqual(0, result.returncode, result.stderr)
                self.assertEqual(ignored, result.stdout.rstrip("\0").split("\0"))

    def test_workspace_guidance_is_self_contained_and_descendant_only(self):
        import re
        contract = "Each workspace knows only itself and its descendants."
        for base in (SOURCE, SOURCE / "src/templates/workspace", SOURCE / "src/templates/project"):
            self.assertIn(contract, (base / "AI.md").read_text())
        for base in (SOURCE / "src/templates/workspace", SOURCE / "src/templates/project"):
            self.assertIn(contract, (base / "guidance/rules/boundaries.md").read_text())
            for guide in base.rglob("*.md"):
                content = guide.read_text()
                self.assertNotRegex(content.lower(), r"\bparent(?:'s)?\s+workspace\b")
                for target in re.findall(r"\]\(([^)]+)\)", content):
                    if "://" in target:
                        continue
                    linked = (guide.parent / target.split("#", 1)[0]).resolve()
                    self.assertTrue(linked.is_relative_to(base.resolve()), (guide, target))
        for guide in (SOURCE / "README.md", SOURCE / "docs/workspace-model.md"):
            self.assertIn(contract, guide.read_text())
            self.assertNotRegex(guide.read_text().lower(), r"\bparent(?:'s)?\s+workspace\b")

    def test_generated_workspace_runs_independently(self):
        destination = self.root / "second workspace"
        self.assertEqual(0, self.run_cli("init", str(destination)))
        self.assertEqual(2, self.run_cli("init", str(destination)))
        required_files = [
            "README.md", "AI.md", "AGENTS.md", ".gitignore", "workspace.json", "WSUP-LICENSE",
            "docs/README.md", "docs/workspace-model.md",
            "guidance/rules/index.md", ".agents/skills/workspace-check/SKILL.md",
            ".wsup/wsup.py",
            ".wsup/wsup_core/cli.py", ".wsup/templates/workspace/README.md",
            ".wsup/templates/project/README.md",
        ]
        for relative in required_files:
            self.assertTrue((destination / relative).is_file(), relative)
        for relative in ("archive", "artifacts", "private", "learning", "research",
                         "reference", "journal", "templates", ".ai", ".work"):
            self.assertFalse((destination / relative).exists(), relative)
        for relative in ("apps", "packages", "tools", "infrastructure", "workbench"):
            self.assertTrue((destination / relative).is_dir(), relative)
        self.assertEqual(
            (SOURCE / "src/wsup.py").read_bytes(),
            (destination / ".wsup/wsup.py").read_bytes())
        for source in (SOURCE / "src/wsup_core").rglob("*.py"):
            bundled = destination / ".wsup/wsup_core" / source.relative_to(SOURCE / "src/wsup_core")
            self.assertEqual(source.read_bytes(), bundled.read_bytes())
        for source in (SOURCE / ".agents/skills").rglob("SKILL.md"):
            for bundled_root in (destination / ".agents/skills", destination / ".wsup/templates/workspace/.agents/skills"):
                bundled = bundled_root / source.relative_to(SOURCE / ".agents/skills")
                self.assertEqual(source.read_bytes(), bundled.read_bytes())
        for command in ["validate", "guidance", "editor"]:
            result = subprocess.run([sys.executable, "-B", ".wsup/wsup.py", command], cwd=destination, capture_output=True)
            self.assertEqual(0, result.returncode, result.stderr)
        self.assertTrue((destination / "AGENTS.md").is_file())
        self.assertTrue((destination / "AI.md").is_file())
        self.assertTrue((destination / "guidance/rules/index.md").is_file())
        self.assertTrue((destination / ".agents/skills/workspace-check/SKILL.md").is_file())
        self.assertFalse(list((destination / ".ai/skills").rglob("SKILL.md")))
        project = destination / ".wsup/templates/project"
        self.assertTrue((project / "AGENTS.md").is_file())
        # Independent guidance has no required paths escaping its own repository.
        import re
        for guide in project.rglob("*.md"):
            for target in re.findall(r"\]\(([^)]+)\)", guide.read_text()):
                linked = (guide.parent / target).resolve()
                self.assertTrue(linked.is_relative_to(project.resolve()))
                self.assertTrue(linked.exists(), target)
        self.assertFalse((destination / ".git").exists())

        nested = self.root / "nested workspace"
        result = subprocess.run(
            [sys.executable, "-B", ".wsup/wsup.py", "init", str(nested)],
            cwd=destination, capture_output=True)
        self.assertEqual(0, result.returncode, result.stderr)
        result = subprocess.run(
            [sys.executable, "-B", ".wsup/wsup.py", "validate"],
            cwd=nested, capture_output=True)
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertTrue((nested / ".wsup/wsup.py").is_file())
        self.assertTrue((nested / ".agents/skills/workspace-check/SKILL.md").is_file())
        for root in (destination, nested):
            for relative in ("WSUP-LICENSE",
                             ".wsup/templates/workspace/WSUP-LICENSE",
                             ".wsup/templates/project/WSUP-LICENSE"):
                self.assertEqual(
                    (SOURCE / "LICENSE").read_bytes(),
                    (root / relative).read_bytes(), relative)

        for path in destination.rglob("*"):
            if path.is_file():
                self.assertNotIn(str(SOURCE).encode(), path.read_bytes(), str(path.relative_to(destination)))

    @unittest.skipUnless(shutil.which("git"), "Git is required for ignore verification")
    def test_independent_project_ignores_local_files(self):
        target = self.root / "standalone"
        self.assertEqual(0, self.run_cli("init", str(target), "--template", "project"))
        subprocess.run(["git", "init", "."], cwd=target, capture_output=True, check=True)
        ignored = [".env", ".env.local", "wsup.local.json", ".work/log", ".wsup/reports/run.json",
                   "nested/.work/log", "nested/__pycache__/module.pyc"]
        tracked = [".env.example", "WSUP-LICENSE", "README.md"]
        for name in ignored + tracked[:1]:
            path = target / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("Synthetic local file")
        for name in ignored + tracked:
            result = subprocess.run(["git", "check-ignore", "-q", name], cwd=target)
            self.assertEqual(0 if name in ignored else 1, result.returncode, name)
        self.assertEqual(
            (SOURCE / "LICENSE").read_bytes(),
            (target / "WSUP-LICENSE").read_bytes())

    @unittest.skipUnless(shutil.which("git"), "Git is required for ownership verification")
    def test_generated_workspace_mixed_git_onboarding(self):
        destination = self.root / "mixed workspace"
        self.assertEqual(0, self.run_cli("init", str(destination)))
        def generated(*args):
            result = subprocess.run(
                [sys.executable, "-B", ".wsup/wsup.py", *args],
                cwd=destination, capture_output=True, text=True)
            self.assertEqual(0, result.returncode, result.stderr)
            return result.stdout
        def git(*args):
            result = subprocess.run(
                ["git", *args], cwd=destination, capture_output=True, text=True)
            self.assertEqual(0, result.returncode, result.stderr)
            return result.stdout

        git("init", ".")
        projects = []
        for name, ownership in (("owned", "workspace"), ("independent", "independent")):
            path = destination / "apps" / name
            generated("init", str(path), "--template", "project")
            projects.append({
                "id": name, "path": f"apps/{name}", "kind": "app",
                "repository": ownership, "purpose": "Onboarding fixture", "owner": "Test",
                "checks": {"test": ["{python}", "-B", "-c",
                    "from pathlib import Path; assert Path('README.md').is_file()"]},
            })
        with (destination / ".gitignore").open("a") as stream:
            stream.write("\n/apps/independent/\n")
        git("init", "apps/independent")
        independent = destination / "apps/independent"
        self.assertEqual(independent.resolve(), Path(git("-C", str(independent), "rev-parse", "--show-toplevel").strip()).resolve())
        self.assertIn("/apps/independent/", git("check-ignore", "-v", "apps/independent/"))
        (destination / "workspace.json").write_text(json.dumps({
            "version": 1, "name": "Mixed fixture", "projects": projects}))
        generated("validate")
        self.assertEqual(projects, json.loads(generated("projects")))
        generated("config")
        result = json.loads(generated("guidance"))
        self.assertEqual(12, len(result["skills"]))
        generated("guidance", "--project", "independent")
        generated("check", "--project", "owned", "--project", "independent")
        generated("editor")
        editor = json.loads((destination / "workspace.code-workspace").read_text())
        self.assertEqual([".", "apps/owned", "apps/independent"], [
            folder["path"] for folder in editor["folders"]])
        self.assertFalse((independent / ".agents/skills").exists())
        self.assertFalse((independent / "tools/wsup.py").exists())
        git("add", "--all")
        tracked = git("ls-files").splitlines()
        for name in ("workspace.json", "apps/owned/README.md", ".wsup/template-state.json",
                     ".agents/skills/project-onboarding/SKILL.md"):
            self.assertIn(name, tracked)
        self.assertFalse(any(name.startswith("apps/independent") for name in tracked))
        self.assertFalse(any(name.startswith(".wsup/reports/") for name in tracked))
        self.assertFalse(any(line.startswith("160000") for line in git("ls-files", "--stage").splitlines()))

if __name__ == "__main__":
    unittest.main()
