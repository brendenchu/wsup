# WSUP

Know what is in your workspace, how to work on each project, and what you have
actually verified.

wsup is a local-first CLI for understanding and maintaining projects that live
in one or many repositories. A `wsup.json` registry records project locations,
owners, relationships and literal task commands. Inspect one project's setup,
run explicitly selected checks, and preview updates to wsup-managed content
without treating independent projects as one build or release.

Starters establish that structure for new workspaces and projects, including
`AGENTS.md`, portable skills and optional agent adapters. Coding-agent use is
optional: wsup never invokes a skill or an AI service and needs no provider,
account or credential. People and assistants use the same local project facts.

**Local-first and independent.** Each workspace knows only itself and its
descendants. Its documentation, guidance, registry, and commands are rooted
locally; nothing is discovered or inherited from a parent directory, and a
nested workspace or project keeps its own self-contained instructions. The CLI
has no third-party runtime dependencies.

## Requirements

Use a trusted native `wsup` binary for your machine. Creating, inspecting,
adding to, and updating workspaces needs no development toolchain. Project
tasks need their own configured tools; wsup passes arguments literally and
never installs tools.

macOS ARM64 is the verified runtime. Unix tasks run under process-group
supervision; on other systems tasks refuse to launch until equivalent cleanup
exists. Cross-compilation does not establish runtime support.

Download the macOS ARM64 binary from the
[releases page](https://github.com/wavykit/wsup/releases). See
[native binary use](docs/native-install.md) for checksum verification,
extraction, invocation by path, replacement, and recovery. There is no
installer, automatic updater, or PATH change.

To build from source, use Go 1.27.3 (pinned in `mise.toml`); see
[Go development](CONTRIBUTING.md#go-development).

The canonical repository is [wavykit/wsup](https://github.com/wavykit/wsup).
Development source now uses the Go module `github.com/wavykit/wsup`.
This path change is unreleased; it is not yet a verified released install channel.
See the [source module transition](docs/native-install.md#source-module-transition)
for historical v0.3.0 installs and release-dependent verification.
Maintainers evaluating current source can use the
[verified commit-pinned candidate procedure](docs/native-install.md#source-candidate-installation);
it is not a released or verified `@latest` channel.

## Try it

The examples use `wsup` for the verified executable; substitute its absolute
path unless you have added it to PATH.

```sh
wsup --version
wsup init /path/to/workspace
wsup init /path/to/project --template project
wsup --root /path/to/workspace validate
wsup --root /path/to/workspace guidance
```

In a terminal, `init` asks for the preset, agents, and editors that flags leave
open (a project asks only for agents); press Enter to accept a default. Without
a terminal, or with `--yes`, it uses the defaults: the `development` preset,
every agent adapter, and every editor adapter.

`init` never overwrites an existing destination. The binary embeds every
template and works without this checkout; a workspace holds `wsup.json`,
`wsup.lock`, and ignored reports, never an executable or a copied catalogue.

Follow the [first-workspace tutorial](docs/getting-started.md) to create a
workspace, register a project, and run a check.

## Work on registered projects

The tutorial's [multi-project example](docs/getting-started.md#6-inspect-and-maintain-multiple-projects-source-candidate)
registers `readme-demo` and `docs-demo` with explicit ownership and documentation
checks. With the verified source candidate, inspect the configuration before
choosing what to execute:

```sh
wsup --root /path/to/workspace projects
wsup --root /path/to/workspace projects docs-demo
wsup --root /path/to/workspace check --project docs-demo --task readme
wsup --root /path/to/workspace update
```

The project view shows the registered owner, task arguments, direct
dependencies/dependents and instruction/skill paths. It does not run anything
or prove a task's tools are installed. Selecting `docs-demo` does not implicitly
check its `readme-demo` dependency; select both explicitly when needed.

The check report records what the selected task actually returned, not the
health of every project. `update` only previews changes to managed starter
content; it does not upgrade the executable or application dependencies.
Review conflicts and keep a matched backup before applying. Existing project
directories can be registered manually after deciding their Git ownership;
`init` creates new destinations, never adopts or overwrites an existing one.

`projects ID` requires the source candidate; the published v0.3.0 binary
supports the no-ID listing, explicit checks and managed-update previews.

## Commands

Run from the workspace root, or put `--root PATH` **before** the command. The
executable's location does not select a workspace, and wsup never searches
parent directories.

| Command | Purpose |
| --- | --- |
| `init DESTINATION` | Create a workspace, or a project with `--template project` |
| `validate` | Validate `wsup.json` and adapter links without running project code |
| `projects [ID]` | List registered projects, or inspect one project's configured metadata (`--json` for a versioned result) |
| `config` | Show effective settings and their sources |
| `guidance` | Check `AGENTS.md` files, skills, markers, and local links (`--project ID` for one project) |
| `modules [ID]` | List built-in modules, or show one module's files and skills |
| `add MODULE...` | Preview adding collections or adapters; `--apply` installs them |
| `update` | Preview managed-content updates; `--apply` writes, `--keep PATH` keeps a customization |
| `migrate` | Preview converting a wsup 0.2.0 workspace or project; `--apply` converts it |
| `check` | Run the check tasks of `--project ID` (repeatable) or `--all` |
| `run PROJECT TASK` | Run one project task in the terminal |
| `vscode` | Preview regenerating `<name>.code-workspace`; `--apply` writes it |
| `schema` | Print the JSON Schema for `wsup.json` |

`init` creates a new destination immediately. `run` and `check` execute
selected tasks, and `check` writes reports. Managed-content commands (`add`,
`update`, `migrate`, and `vscode`) preview by default and write only with
`--apply`, when the plan has no conflicts. See the
[command reference](templates/modules/workspace-base/files/docs/workspace-model.md#command-reference)
for flags, output, and exit codes.

To see how one project is configured without running anything:

```sh
wsup projects api
wsup projects api --json
```

The view shows owner/location, configured tasks, direct relationships and local
instruction/skill paths. It does not check task prerequisites or prove that an
AI client loads the guidance. See
[single-project inspection](templates/modules/workspace-base/files/docs/workspace-model.md#single-project-inspection)
for scope, versioned JSON and errors.

Tasks marked `"check": true` in `wsup.json` run under `wsup check`. Commands
run locally with your permissions, without a shell or argument substitution;
review them before running. Reports go to the ignored
`workbench/.local/wsup/reports/` and hold results and hashes, not command
output or arguments; `--show-output` prints output in the terminal.

## Workspaces, modules, and projects

Everything wsup installs is a **module**: the base, opt-in **collections**
(folders with their own README, AGENTS.md, and sometimes skills), **agent
adapters**, **editor adapters**, and **presets** that select collections at
init. A **project** is a member registered in `wsup.json`.

Every workspace has `docs/`, `workbench/`, `AGENTS.md`, `.agents/skills/`,
`wsup.json`, and `wsup.lock`. The default `development` preset adds `apps/`,
`packages/`, `tools/`, and `infra/`; `--preset none` gives the base only. Any
collection can be selected at init or added later:

| Group | Collections |
| --- | --- |
| Projects | `apps`, `packages`, `tools`, `infra`, `sandbox`, `templates`, `forks` |
| Knowledge and lifecycle | `research`, `reference`, `decisions`, `learning`, `journal`, `archive`, `releases` |
| Work and making | `issues`, `ai`, `design`, `content`, `data` |

```sh
wsup init /path/to/workspace --preset none --collection research --collection journal
wsup --root /path/to/workspace modules --kind collection
wsup --root /path/to/workspace add decisions
wsup --root /path/to/workspace add decisions --apply
```

Additions never replace existing files: any conflict blocks the whole apply,
and `--keep PATH` keeps an owner file at a path the module installs. A child
workspace never inherits its parent's selections. Folder names describe
placement, not Git ownership, permissions, backup, or retention.

The [module catalogue](templates/modules/workspace-base/files/docs/workspace-model.md#module-catalogue)
lists each collection's files and skills, and the
[workspace model](templates/modules/workspace-base/files/docs/workspace-model.md)
covers the registry, tasks, settings, Git ownership, and updates.

The registry does not imply a shared repository, build, or release. For an
independent project repository, ignore its directory in the workspace and mark
it `independent`; see
[Git ownership](templates/modules/workspace-base/files/docs/workspace-model.md#git-ownership).

## Agents and skills

The root `AGENTS.md` holds the workspace's rules and two generated sections,
Layout and Skills, between `<!-- wsup:begin NAME -->` markers; everything
outside the markers is yours, and updates merge around your edits. Each
collection folder has its own short `AGENTS.md`.

Every workspace has seven base skills: check-workspace, run-checks,
update-workspace, onboard-project, plan-work, recap-work, and
maintain-agents-md. The project collections add five code-change skills
(make-change, review-change, diagnose-issue, write-commit, infer-conventions),
installed once however many are selected. Some opt-in collections bring
their own skills, such as `record-decision` with `decisions`; the module
catalogue lists them. The project starter has the five code-change skills plus
plan-work.

Agent adapters point agents that don't read `AGENTS.md` or `.agents/skills/`
natively at the same files. They hold no instructions of their own:

| Adapter | Adds |
| --- | --- |
| `claude` | `CLAUDE.md` importing `AGENTS.md`, a `.claude/skills` link to `.agents/skills`, and an ignore entry for `.claude/settings.local.json` |
| `gemini` | `GEMINI.md` importing `AGENTS.md` |

`init` installs every adapter; choose with `--agent NAME` (repeatable) or skip
with `--agent none`, and add one later with `wsup add`. The `vscode` editor
adapter generates `<name>.code-workspace` from `wsup.json` for VS Code, Cursor,
Windsurf, and VSCodium. Agents that read the shared files natively, such as
Codex, need no adapter.

`wsup guidance` checks structure, metadata, markers, and links; it does not
prove that a client loaded or invoked a skill. See
[AI guidance](docs/ai-guidance.md) for the client compatibility table.

## Updates and upgrading

`wsup.lock` records the installed modules and the hash of every file wsup
wrote. Commit it. `wsup update` compares your files with the invoked binary's
templates: unedited files are refreshed, edited ones are reported as conflicts
until you `--keep` or resolve them, and nothing is downloaded. Before applying,
keep a backup of your files and `wsup.lock`; see
[updates and recovery](templates/modules/workspace-base/files/docs/workspace-model.md#generated-files-and-updates).

Workspaces and projects created by wsup 0.2.0 convert with `wsup migrate`;
follow the [migration guide](docs/migrating.md).

## This development checkout

This repository holds only the CLI source and the content it generates:

| Directory | Role |
| --- | --- |
| [cmd/wsup](cmd/wsup/) | Native executable entry point. |
| [internal](internal/) | Go packages and colocated regression tests. |
| [templates](templates/) | The single source for generated workspaces, projects, modules, and skills, embedded in the binary. |
| [schema](schema/) | The JSON Schema for `wsup.json`. |
| [docs](docs/) | Product documentation: install, getting started, migration, AI guidance, releases. |

It is not itself a wsup workspace. Contributor and agent instructions are in
[CONTRIBUTING.md](CONTRIBUTING.md) and [AGENTS.md](AGENTS.md).

## Contributing and releases

See [CONTRIBUTING.md](CONTRIBUTING.md) for development and verification,
[CHANGELOG.md](CHANGELOG.md) for changes by version, and the
[release guide](docs/releases.md) for versioning and tags.

## License

WSUP is licensed under the [MIT License](LICENSE). When redistributing WSUP
code or substantial template content, retain the copyright and permission
notice. Generated workspaces and projects carry no license file; choosing a
license for your own projects is up to you.
