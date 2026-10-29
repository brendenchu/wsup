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

No Go toolchain, package installation, service, or AI client is needed
to initialize or inspect this workspace. Project checks supply their own tools.
Native check execution is verified on macOS ARM64. Other runtime platforms are
not certified; non-Unix check execution explicitly refuses launch.

Run from this directory or put `--root /path/to/workspace` before the command.
The executable's location does not select a workspace. With the `vscode`
editor adapter (installed by default), `<name>.code-workspace` is generated
from `workspace.json` and refreshed by `update`; `editor` is optional, writes
the same document to a new file, and refuses existing files. See the [workspace model](docs/workspace-model.md)
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
Arguments are never substituted; missing tools are explicit launch errors,
never installed automatically.

## Layout and local work

Every workspace has `docs/` and `workbench/`. The `development` preset, used by
default, adds the `apps/`, `packages/`, `tools/`, and `infra/` collections; `wsup init --preset none` creates only the base. Use homes by
responsibility without requiring every home to be populated. Shareable human drafts can live in `workbench/`; local plans and
notes belong under ignored `workbench/.local/`, created on demand. Ignored does
not mean backed up, disposable, or secure. Promote approved knowledge to the
owning project's maintained documentation.

Every other collection is created only when selected:

| Group | Collections |
| --- | --- |
| Projects | `apps`, `packages`, `tools`, `infra`, `sandbox`, `templates`, `forks` |
| Knowledge and lifecycle | `research`, `reference`, `decisions`, `learning`, `journal`, `archive`, `releases` |
| Work and making | `issues`, `ai`, `design`, `content`, `data` |


```sh
wsup init /path/to/new-workspace --collection archive --collection journal
wsup init /path/to/new-workspace --preset none --collection research
wsup collections
wsup collections --add decisions --add issues
wsup collections --add decisions --add issues --apply
```

Selections are deduplicated and retained for template updates, not inherited by
child workspaces. Existing unmanaged scaffold files are conflicts even when
identical; all conflicts block additions. Already-selected managed files retain
customizations; review them with `update`. Additions create documentation (decisions,
issues, and journal also include a template), not permissions, backups,
encryption, or retention policies. A collection's ignore entries go in the
`.gitignore` block marked `# wsup:begin ignore` / `# wsup:end ignore`. A directory or ignore rule is not an access-control
boundary; keep restricted records out of documentation and Git.

## Generation, updates, and recovery

```sh
wsup init /path/to/another-workspace
wsup init /path/to/new-project --template project
wsup init /path/to/scripted-workspace --yes
```
In a terminal, `init` asks for the preset, agents, and editors (a project:
agents only) that flags leave open; press Enter to accept a default. Without a
terminal, or with `--yes`, it uses the defaults without asking.

Destinations must not exist, even as empty directories or dangling symlinks.
Relative destinations use the current directory, not `--root`. Initialization
uses templates embedded in the executable, not this workspace's local edits.
No runtime, executable, or catalogue is installed in the workspace. Relocation
requires a compatible separately installed binary on the new machine.

The `.wsup/template-state.json` schema is version 2. Track it with the workspace
files; ignore reports only. `update` previews managed content from the invoked
binary; `update --apply`
writes only a conflict-free plan. Repeat `--keep PATH` for eligible same-path
customizations, preserving their old baseline hashes. Registry bytes are
workspace-owned; omitted homes and unknown/report/local/project bytes survive.
Only schema-2 workspaces are supported. Unsupported state fails without writes;
there is no in-place conversion. Do not edit schema numbers to force compatibility.
Binary replacement and workspace-content updates are separate operations,
never automatic downloads.

Update I/O failures may leave partial content with old state and explicitly
report that possibility; no multi-file transaction is promised. Initialization
failures explicitly identify a partial destination. Inspect and
recover it before retrying; partial output is retained to avoid deleting
concurrent owner content. Back up customizations and state before any future
update. Nothing is committed, installed globally, or published
automatically.

## Guidance

Read [AGENTS.md](AGENTS.md): it holds this workspace's instructions, a
generated layout of the selected collections, and a generated table of the
installed skills. Each collection folder has its own AGENTS.md with that
folder's rules. wsup writes only between `<!-- wsup:begin NAME -->` and
`<!-- wsup:end NAME -->` markers; everything else in AGENTS.md is yours.

Portable skills are under `.agents/skills/`: seven base skills, plus
code-change skills when a project collection is selected. Agent adapters
(`CLAUDE.md`, `GEMINI.md`, and the `.claude/skills` link to `.agents/skills/`)
point those agents at the same files; see the
[workspace model](docs/workspace-model.md#agent-adapters-and-the-skills-link).
Clients may support these locations or need an explicit `SKILL.md` path. Verify actual loading separately: `guidance` checks
local structure, metadata, markers, and links, not client discovery or
invocation. Independent projects keep their own instructions.
