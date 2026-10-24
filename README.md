# WSUP

A small, local-first foundation for organizing projects that may live in one or
many repositories. It provides a project registry, optional VS Code workspace
generation, explicit local checks, and a portable workspace/project starter.
It has no third-party runtime dependencies or required AI service.

## Requirements

Python 3.9 or newer. Registered projects may use any language or toolchain; each
project owns its dependencies and commands.

An existing macOS developer-tools Python 3.9 can run WSUP; check your installed
version with `python3 --version`. There is no need to replace the system Python.
WSUP compatibility does not extend upstream Python security maintenance.

## Try it

Run these from this directory:

```sh
python3 -B tools/wsup.py validate
python3 -B tools/wsup.py projects
python3 -B tools/wsup.py check --project wsup
```

Create a workspace or a project starter at a new path:

```sh
python3 -B tools/wsup.py init /path/to/workspace
python3 -B tools/wsup.py init /path/to/project --template project
```

`init` never overwrites an existing destination. A workspace starter includes
the CLI and both templates, so it can be used without this checkout.

## Use the CLI

Run `python3 -B tools/wsup.py --help` for command options. The main commands:

| Command | Purpose |
| --- | --- |
| `validate` | Validate `workspace.json` without running project code |
| `projects` | List registered projects |
| `config` | Show effective settings and their sources |
| `guidance` | Check workspace instructions and local links |
| `guidance --project ID` | Check one project's instructions |
| `editor` | Create a relative-path VS Code workspace file |
| `check --project ID` | Run only that project's configured checks |
| `update` | Preview available template updates |
| `update --apply` | Apply conflict-free updates |

Use `--project ID` repeatedly to select more projects and `--check NAME`
repeatedly to choose checks. Review each configured command before running it:
commands execute locally with your permissions. The CLI does not install
dependencies on its own, invoke a shell, or automatically check dependent
projects.

Check reports are written under the ignored `.work/` directory. They contain
results and hashes rather than command output or arguments. Use `--show-output`
to display output in the terminal.

## Workspace layout

`apps/`, `packages/`, `tools/`, `infrastructure/`, `research/`, and `templates/`
are optional responsibility groups. `docs/` holds durable guidance; `journal/`
is an optional work record. Register projects in `workspace.json`; projects own
their code, tests, dependencies, setup and required instructions.

The registry does not imply a shared Git repository, build, dependency install,
or release process. See the [workspace model](docs/workspace-model.md) for the
registry, settings, checks, reports and update behavior. See [AI guidance](docs/ai-guidance.md)
for the portable instruction kit and optional client integration.

For independent project repositories, add each project's directory to the
workspace `.gitignore` and mark it `independent` in the registry. Workspace-owned
projects stay tracked normally. See [Git ownership](docs/workspace-model.md)
for setup and existing-file precautions.

## Optional AI skills

Workspace initialization includes 12 portable skills in `.agents/skills/`.
Clients supporting this shared location need no installation command. Open the
workspace root and inspect the client's available skills; otherwise explicitly
ask the assistant to read a skill's `SKILL.md` and follow its procedure.

Run `python3 -B tools/wsup.py guidance` to check local structure and links.
That does not prove a client loaded or invoked a skill. Read
[AI.md](AI.md) and the rule index separately for workspace instructions.
No client adapter, global installation, credential, or model configuration is
created. There is no `skills install` command.

Use the normal template-update preview/apply flow to refresh bundled content;
it does not download a new release. To disable skill use, use the client's
controls rather than deleting canonical files and breaking rule-index links.
See [AI guidance](docs/ai-guidance.md) for the dated compatibility matrix and
[workspace model](docs/workspace-model.md) for safe legacy upgrades.

## Contributing and releases

See [CONTRIBUTING.md](CONTRIBUTING.md) for development and verification,
[CHANGELOG.md](CHANGELOG.md) for version milestones, and the
[release guide](docs/releases.md) for versioning and annotated tags.
These are maintainer documents, not generated starter files.

## License

WSUP is licensed under the [MIT License](LICENSE).
When redistributing WSUP code or substantial template content, retain the
copyright and permission notice. Both starters include a `WSUP-LICENSE` notice,
kept in sync with this repository's license. This covers WSUP content and does
not choose a license for your own projects.
