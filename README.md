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

Use a trusted native `wsup` binary compatible with your machine. Normal
initialization, inspection, collections, and updates need no development toolchain.
Project checks require their configured tools and pass arguments literally
without substitution. WSUP never installs those tools automatically.

Runtime verification is currently macOS ARM64. Unix checks use process-group
supervision; non-Unix checks explicitly refuse launch pending equivalent
descendant cleanup. Cross-compilation does not establish runtime support.

Download the compiled macOS ARM64 binary from the
[WSUP v0.2.0 release](https://github.com/brendenchu/wsup/releases/tag/v0.2.0).
Download users do not need Go or this checkout. See
[native binary use](docs/native-install.md) for download links, checksum
verification, extraction, explicit-path invocation, replacement, and recovery.
There is no binary installer, automatic updater, or automatic PATH change.

To build from source instead, use Go 1.27.3, pinned in `mise.toml`; see
[Go development](CONTRIBUTING.md#go-development). Development tests also require
Git for ownership and reporting assertions. Only version 1 of `wsup.lock` is
supported; unsupported formats fail without writes.

## Try it

After verifying and extracting the release archive, invoke its executable by
absolute path to create a workspace or project at a new destination:

```sh
/absolute/path/to/wsup_0.2.0_darwin_arm64/wsup --version
/absolute/path/to/wsup_0.2.0_darwin_arm64/wsup init /path/to/workspace
/absolute/path/to/wsup_0.2.0_darwin_arm64/wsup init /path/to/project --template project
```

In a terminal, `init` asks for the preset, agents, and editors (a project:
agents only) that flags leave open; press Enter to accept a default. Without a
terminal, or with `--yes`, it uses the defaults without asking.

Inspect the new workspace without installing anything or changing PATH:

```sh
/absolute/path/to/wsup_0.2.0_darwin_arm64/wsup --root /path/to/workspace validate
/absolute/path/to/wsup_0.2.0_darwin_arm64/wsup --root /path/to/workspace projects
/absolute/path/to/wsup_0.2.0_darwin_arm64/wsup --root /path/to/workspace guidance
```

`init` never overwrites an existing destination. A compiled executable embeds
all templates and works without this checkout. Build outside the tracked tree
with `go build -o /path/to/wsup ./cmd/wsup`, then invoke `/path/to/wsup` (or
`wsup` if separately made available on PATH). Workspaces contain `wsup.json`,
`wsup.lock`, and ignored reports, not an executable, runtime, or copied catalogue. Relocation
requires a compatible external binary. Agent instructions live in `AGENTS.md`
(plus one per collection folder) and portable skills in `.agents/skills/`.
Generated workspaces include the `development` preset's homes by default, with
other collections selected explicitly; see the workspace layout below.
The project starter contains project documentation, guidance, its own skills,
ignore rules, and template state, but no bundled CLI or workspace registry.

Follow the [first-workspace tutorial](docs/getting-started.md) for a complete
create/register/check walkthrough, or see
[optional PATH setup](docs/native-install.md#optional-path-setup) to use `wsup`
without its full path.

## Use the CLI

Run the downloaded or source-built executable with `--help`, or
`go run ./cmd/wsup --help` in this checkout. Examples below use `wsup` for the
trusted executable; substitute its absolute path unless you separately added it
to PATH. The main commands:

| Command | Purpose |
| --- | --- |
| `validate` | Validate `wsup.json` without running project code |
| `projects` | List registered projects |
| `config` | Show effective settings and their sources |
| `guidance` | Check workspace instructions and local links |
| `guidance --project ID` | Check one project's instructions |
| `editor` | Create a relative-path VS Code workspace file (the `vscode` adapter generates one at init) |
| `check --project ID` | Run only that project's configured checks |
| `modules` | List built-in modules: collections, adapters, and presets |
| `add MODULE...` | Preview adding collections or adapters |
| `add MODULE... --apply` | Add them without replacing existing files |
| `update` | Preview available template updates |
| `update --apply` | Apply conflict-free updates |
| `update --keep PATH` | Retain a same-path customization (repeat for each path) |

Run from the workspace root, or supply the global `--root` option **before**
the command. Executable location does not select the target workspace, and WSUP
does not search parent directories:

```sh
/path/to/wsup --root /path/to/workspace projects
wsup check --help
```

See the [command reference](templates/modules/workspace-base/files/docs/workspace-model.md#command-reference) for
flags, output formats, exit codes, and root/destination behavior.

Checks are the project tasks marked `"check": true` in `wsup.json`. Use
`--project ID` repeatedly to select more projects, or `--all`, and `--task NAME`
repeatedly to choose checks. Review each configured command before running it:
commands execute locally with your permissions. The CLI does not install
dependencies on its own, invoke a shell, or automatically check dependent
projects. Kept customizations retain their existing baselines and remain visible
for review in later update previews.

Check reports are written under the ignored `workbench/.local/wsup/reports/` directory. They
contain results and hashes rather than command output or arguments. Use
`--show-output` to display output in the terminal.

Only workspaces with a version 1 `wsup.lock` can update. Never edit version
numbers to bypass validation. Before
applying updates, preserve a matched backup of files and `wsup.lock`.
See [updates and recovery](templates/modules/workspace-base/files/docs/workspace-model.md#generated-files-and-updates).

## Workspace layout

Every generated workspace includes `docs/` and `workbench/`, alongside the
workspace-owned `AGENTS.md`, `.agents/skills/`, `wsup.json`, and `wsup.lock`. The
default `development` preset adds the `apps/`, `packages/`, `tools/`, and
`infra/` collections; `--preset none` creates the base only. Every other
collection is a CLI-selectable addition:

| Group | Collections |
| --- | --- |
| Projects | `apps`, `packages`, `tools`, `infra`, `sandbox`, `templates`, `forks` |
| Knowledge and lifecycle | `research`, `reference`, `decisions`, `learning`, `journal`, `archive`, `releases` |
| Work and making | `issues`, `ai`, `design`, `content`, `data` |

Folder
names describe placement, not Git ownership, project kind, sensitivity
controls, backup, or retention guarantees. See the
[workspace model](templates/modules/workspace-base/files/docs/workspace-model.md#default-layout-and-optional-collections)
for guidance. The template catalogue is embedded in the external binary, never
read from workspace-local overrides. Register projects in
`wsup.json`; projects own their code, tests, dependencies, setup and
required instructions.

Select optional collections during initialization, or add them later:

```sh
wsup init /path/to/workspace --collection archive --collection releases
wsup --root /path/to/workspace add decisions
wsup --root /path/to/workspace add decisions --apply
```

Each collection creates a purpose-and-boundaries README; decisions, issues,
and journal also include a template. Existing-workspace additions preview by default; any
conflicting scaffold file blocks the whole apply.
An already-managed scaffold is left untouched, including local customizations.
Selections are recorded in template state so normal updates include them.
They are not inherited when that workspace generates another workspace; select
the desired collections explicitly for each new workspace. Project starters
do not accept collection selections.

These commands do not change permissions, encryption, backup, sync, indexing,
or retention settings. A collection's ignore entries (such as `data/**/*.local.*`)
go in the `.gitignore` block between `# wsup:begin ignore` and
`# wsup:end ignore`; adding a collection rewrites `.gitignore` only while it is
unedited and otherwise reports a conflict.

The registry does not imply a shared Git repository, build, dependency install,
or release process. See the [workspace model](templates/modules/workspace-base/files/docs/workspace-model.md) for the
registry, settings, checks, reports and update behavior. See [AI guidance](docs/ai-guidance.md)
for the portable instruction kit and optional client integration.

For independent project repositories, add each project's directory to the
workspace `.gitignore` and mark it `independent` in the registry. Workspace-owned
projects stay tracked normally. See [Git ownership](templates/modules/workspace-base/files/docs/workspace-model.md)
for setup and existing-file precautions.

## This development checkout

This repository holds only the CLI source and the content it generates:

| Directory | Role |
| --- | --- |
| [cmd/wsup](cmd/wsup/) | Native executable entry point. |
| [internal](internal/) | Go packages and colocated regression tests. |
| [templates](templates/) | The single source for generated workspaces, projects, collections, skills, and guidance, embedded directly in the binary. |
| [docs](docs/) | Product documentation: install, getting started, AI guidance, releases. |

It is not itself a wsup workspace: it has no registry, workspace guidance, or
skills of its own. Contributor and agent instructions are in
[CONTRIBUTING.md](CONTRIBUTING.md) and [AGENTS.md](AGENTS.md).

## Optional AI skills

Every workspace includes seven base skills in `.agents/skills/` (check-workspace,
run-checks, update-workspace, onboard-project, plan-work, recap-work,
maintain-agents-md). The project collections (`apps`, `packages`, `tools`,
`infra`, `sandbox`, `templates`, `forks`) add five code-change skills
(make-change, review-change, diagnose-issue, write-commit, infer-conventions),
installed once however many are selected. The project starter gets those five
plus plan-work.
Clients supporting this shared location need no installation command. Open the
workspace root and inspect the client's available skills; otherwise explicitly
ask the assistant to read a skill's `SKILL.md` and follow its procedure.

Run `wsup guidance` in a generated workspace to check local structure and
links. That does not prove a client loaded or invoked a skill. A generated
workspace's `AGENTS.md` holds its instructions and a generated table of its
installed skills.
`init` also installs agent adapters: `CLAUDE.md` and a `.claude/skills` link
to `.agents/skills/` for Claude Code, and `GEMINI.md` for Gemini CLI. Choose
them with `--agent NAME` (repeatable) or skip them with `--agent none`. No
global installation, credential, or model configuration is created, and there
is no `skills install` command.

Use the normal template-update preview/apply flow to refresh bundled content;
it does not download a new release. To disable skill use, use the client's
controls rather than deleting skill files that AGENTS.md links to.
See [AI guidance](docs/ai-guidance.md) for the dated compatibility matrix and
[workspace model](templates/modules/workspace-base/files/docs/workspace-model.md) for safe managed-content updates.

## Contributing and releases

See [CONTRIBUTING.md](CONTRIBUTING.md) for development and verification,
[CHANGELOG.md](CHANGELOG.md) for version milestones, and the
[release guide](docs/releases.md) for versioning and annotated tags.
These are maintainer documents, not generated starter files.

## License

WSUP is licensed under the [MIT License](LICENSE).
When redistributing WSUP code or substantial template content, retain the
copyright and permission notice. Generated workspaces and projects carry no
license file: the license applies to this repository, and choosing a license
for your own projects is up to you.
