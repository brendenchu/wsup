# WSUP

A small, local-first foundation for organizing projects that may live in one or
many repositories. It provides a project registry, optional VS Code workspace
generation, explicit local checks, and a portable workspace/project starter.
WSUP supports AI and non-AI workflows without requiring a particular provider
or tool. It favors open, widely supported formats and interfaces, while keeping
service-specific integrations optional. The core should remain useful on its
own, and integrations should be replaceable so people can use the tools that
work for them without making those tools requirements for everyone. The CLI has
no third-party runtime dependencies or required AI service.

**Workspace independence:** Each workspace knows only itself and its descendants.
Its documentation, guidance, registry, and commands are rooted locally.
Descendant workspaces carry their own self-contained instructions; no workspace
context is discovered or inherited from outside the current root.

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
the CLI and both templates, so it can be used without this checkout. Inside that
generated workspace, use `python3 -B .wsup/wsup.py ...`. The installed runtime,
single template catalogue, state, and reports live under `.wsup/`; shared rules
live in `guidance/rules/` and portable skills remain in `.agents/skills/`.
Generated workspaces include a focused set of development homes, with optional
collections selected explicitly; see the workspace layout below.

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
| `collections` | List available optional collection scaffolds |
| `collections --add NAME` | Preview adding a collection; repeat `--add` to select more |
| `collections --add NAME --apply` | Add scaffolds without replacing existing files |
| `update` | Preview available template updates |
| `update --apply` | Apply conflict-free updates |
| `update --keep PATH` | Retain a same-path customization (repeat for each path) |

Use `--project ID` repeatedly to select more projects and `--check NAME`
repeatedly to choose checks. Review each configured command before running it:
commands execute locally with your permissions. The CLI does not install
dependencies on its own, invoke a shell, or automatically check dependent
projects. Kept customizations retain their existing baselines and remain visible
for review in later update previews.

Check reports are written under the ignored `.wsup/reports/` directory. They
contain results and hashes rather than command output or arguments. Use
`--show-output` to display output in the terminal.

## Workspace layout

Every generated workspace includes `apps/`, `packages/`, `tools/`,
`infrastructure/`, `docs/`, and `workbench/`, alongside the workspace-owned
`guidance/`, `.agents/skills/`, and `.wsup/` foundation. This baseline focuses
on organizing development work, not prescribing a knowledge-management workflow.
`learning/`, `research/`, `reference/`, and `journal/` are CLI-selectable
additions, as are `archive/`, `artifacts/`, and `private/`. Folder
names describe placement, not Git ownership, project kind, sensitivity
controls, backup, or retention guarantees. See the
[workspace model](docs/workspace-model.md#default-layout-and-optional-collections)
for guidance. The installed template catalogue is tool-owned under
`.wsup/templates/`, not a root project grouping. Register projects in
`workspace.json`; projects own their code, tests, dependencies, setup and
required instructions.

Select optional collections during initialization, or add them later:

```sh
python3 -B tools/wsup.py init /path/to/workspace --collection archive --collection artifacts
python3 -B tools/wsup.py --root /path/to/workspace collections --add private
python3 -B tools/wsup.py --root /path/to/workspace collections --add private --apply
```

Each collection creates a purpose-and-boundaries README; journal also includes
an entry template. Existing-workspace additions preview by default; any
conflicting scaffold file blocks the whole apply.
An already-managed scaffold is left untouched, including local customizations.
Selections are recorded in template state so normal updates include them.
They are not inherited when that workspace generates another workspace; select
the desired collections explicitly for each new workspace. Project starters
do not accept collection selections.

These commands do not change ignore rules, permissions, encryption, backup,
sync, indexing, or retention settings. In particular, the private scaffold is
not protected storage.

The registry does not imply a shared Git repository, build, dependency install,
or release process. See the [workspace model](docs/workspace-model.md) for the
registry, settings, checks, reports and update behavior. See [AI guidance](docs/ai-guidance.md)
for the portable instruction kit and optional client integration.

For independent project repositories, add each project's directory to the
workspace `.gitignore` and mark it `independent` in the registry. Workspace-owned
projects stay tracked normally. See [Git ownership](docs/workspace-model.md)
for setup and existing-file precautions.

## This development checkout

The source root uses the same six development homes, with source-specific
responsibilities rather than a second installed workspace:

| Directory | Role here |
| --- | --- |
| [apps](apps/), [packages](packages/), [infrastructure](infrastructure/) | Baseline homes, currently without projects or services. Their READMEs explain the boundaries; no example projects are required. |
| [tools](tools/) | The WSUP CLI implementation and development entry point. In generated workspaces the runtime lives under `.wsup/` instead. |
| [docs](docs/) | Maintained WSUP product and contributor documentation. |
| [workbench](workbench/) | Reviewed human-authored development drafts, separate from maintained docs and disposable local notes. |
| [templates](templates/) | Product source for workspace, project, and optional collection scaffolds; not this checkout's own workspace content. |
| [tests](tests/) | CLI and starter regression tests. |

The registry describes this repository as one WSUP tool project. Guidance and
portable skills live at the root; `.wsup/reports/` holds ignored check reports,
not an installed runtime. Use ignored `.work/` for session logs and disposable
investigations.

The unused legacy research and journal scaffolds have been retired from this
checkout; their distributable versions remain in the optional catalogue.
No learning, reference, archive, artifacts, or private collection is needed
merely because WSUP offers one. Introduce optional homes here only for actual
development work, with an explicit purpose and owner. Do not run consumer
template updates over the source root.

## Optional AI skills

Workspace initialization includes 12 portable skills in `.agents/skills/`.
Clients supporting this shared location need no installation command. Open the
workspace root and inspect the client's available skills; otherwise explicitly
ask the assistant to read a skill's `SKILL.md` and follow its procedure.

Run `python3 -B tools/wsup.py guidance` in this source checkout, or
`python3 -B .wsup/wsup.py guidance` in an initialized workspace, to check
local structure and links.
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
