# Workspace

Organize related projects without requiring a shared repository, build,
language, or release cycle. The registry describes project ownership and
explicit local checks. Keep integrations optional and replaceable.

Each workspace knows only itself and its descendants. Documentation, guidance,
registry, and commands are scoped to this root. Descendant workspaces have
their own self-contained instructions; there is no upward discovery or policy
inheritance.

## Start

Use a separately installed, trusted native `wsup` executable:

```sh
wsup --version
wsup validate
wsup projects
wsup guidance
wsup editor
```

No Python, Go toolchain, package installation, service, or AI client is needed
to initialize or inspect this workspace. Project checks supply their own tools.
Native check execution is verified on macOS ARM64. Other runtime platforms are
not certified; non-Unix check execution explicitly refuses launch.

Run from this directory or put `--root /path/to/workspace` before the command.
The executable's location does not select a workspace. `editor` is optional
and refuses existing files. See the [workspace model](docs/workspace-model.md)
for command, registry, check, report, ownership, and recovery contracts.

## Projects and checks

Add projects to `workspace.json` with unique IDs and paths, a purpose, owner,
kind, intended repository ownership, and optional dependencies and checks.
Each project needs its own README, setup, tests, dependencies, and instructions.
For an independent repository, ignore that exact project directory in this
workspace before staging; registry metadata alone does not configure Git.

```sh
wsup check --project PROJECT_ID
wsup check --project PROJECT_ID --check test --show-output
wsup config
```

Review commands before running them: checks run with your permissions. They use
literal arguments, no interactive stdin, and bounded captured output; dependencies
are not automatically selected. Reports live under ignored `.wsup/reports/`.
An exact `{python}` argument resolves to `python3` on PATH; missing tools are
explicit launch errors, never installed automatically.

## Layout and local work

Default homes are `apps/`, `packages/`, `tools/`, `infrastructure/`, `docs/`,
and `workbench/`. Use them by responsibility without requiring every home to be
populated. Shareable human drafts can live in `workbench/`; local plans and
notes belong under ignored `workbench/.local/`, created on demand. Ignored does
not mean backed up, disposable, or secure. Promote approved knowledge to the
owning project's maintained documentation.

Knowledge and lifecycle homes (`archive`, `artifacts`, `journal`, `learning`,
`private`, `reference`, `research`) are not created by default. Select explicitly:

```sh
wsup init /path/to/new-workspace --collection archive --collection journal
wsup collections
wsup collections --add private --add artifacts
wsup collections --add private --add artifacts --apply
```

Selections are deduplicated and retained for template updates, not inherited by
child workspaces. Existing unmanaged scaffold files are conflicts even when
identical; all conflicts block additions. Already-selected managed files retain
customizations; review them with `update`. Additions create documentation (journal
also includes a template), not permissions, ignore rules, backups, encryption,
or retention policies. A directory or ignore rule is not an access-control
boundary; keep restricted records out of documentation and Git.

## Generation, updates, and recovery

```sh
wsup init /path/to/another-workspace
wsup init /path/to/new-project --template project
```

Destinations must not exist, even as empty directories or dangling symlinks.
Relative destinations use the current directory, not `--root`. Initialization
uses templates embedded in the executable, not this workspace's local edits.
No runtime, executable, or catalogue is installed in the workspace. Relocation
requires a compatible separately installed binary on the new machine.

The `.wsup/template-state.json` schema is version 2. Track it with the workspace
files; ignore reports only. Python cannot update this native state. Native `update` previews managed content from the invoked binary; `update --apply`
writes only a conflict-free plan. Repeat `--keep PATH` for eligible same-path
customizations, preserving their old baseline hashes. Registry bytes are
workspace-owned; omitted homes and unknown/report/local/project bytes survive.
Migration from Python-managed workspaces is not yet available;
do not use Python's update command on this workspace. Binary replacement and
workspace-content updates are separate operations, never automatic downloads.

Update I/O failures may leave partial content with old state and explicitly
report that possibility; no multi-file transaction is promised. Initialization
failures explicitly identify a partial destination. Inspect and
recover it before retrying; partial output is retained to avoid deleting
concurrent owner content. Back up customizations and state before any future
migration or update. Nothing is committed, installed globally, or published
automatically.

## Guidance

Read [AI.md](AI.md) and `guidance/rules/index.md`. All 12 portable skills are
under `.agents/skills/`; no adapter installation is required by WSUP. Clients
may support that location or need an explicit `SKILL.md` path. Verify actual
loading separately: `guidance` checks local structure, metadata, and links,
not client discovery or invocation. Independent projects keep their own kits.

## WSUP attribution

Starter content and guidance are covered by [WSUP-LICENSE](WSUP-LICENSE).
Retain that notice when redistributing WSUP content. It does not choose a
license for your own projects.
