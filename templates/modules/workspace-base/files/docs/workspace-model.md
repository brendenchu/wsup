# Workspace model

## Glossary

| Term | Meaning |
| --- | --- |
| Workspace | A root with `wsup.json` and `wsup.lock` that groups projects; it knows only itself and its descendants. |
| Project | A member registered in `wsup.json`: a directory with its own README, owner, and tasks. A project starter (`init --template project`) is a standalone project root with `wsup.lock` but no registry. |
| Module | An installable unit with a manifest: the base, a collection, an agent or editor adapter, or a preset. |
| Base | The module every starter gets: `workspace-base` or `project-base`. |
| Collection | An opt-in workspace folder with its own README, AGENTS.md, and sometimes skills or ignore entries. |
| Adapter | A module that points one agent or editor at the shared files (`claude`, `gemini`, `vscode`). It holds no instructions. |
| Preset | A named list of collections, used only by `init`; the workspace records the collections, never the preset. |
| Skill | A portable procedure in `.agents/skills/<name>/SKILL.md`, installed once when any selected module lists it. |
| Task | A named command of a project in `wsup.json`; a check task (`"check": true`) also runs under `wsup check`. |
| Managed file | A file wsup wrote and records in `wsup.lock`, so `update` can tell your edits from its changes. |

## Installation and ownership

Each workspace knows only itself and its descendants. Its registry, guidance,
documentation, and commands describe its own root. No parent discovery,
configuration inheritance, or relationship to the generating workspace exists.
Descendant projects/workspaces retain self-contained instructions.

Use a separately installed native `wsup` executable with embedded templates.
The workspace contains managed files, `wsup.json`, and `wsup.lock`, not a CLI
runtime, binary, or copied catalogue. Reports are under the ignored
`workbench/.local/wsup/reports/`.
The Go toolchain is not a runtime requirement for initialization
or inspection. Checks require their configured executables. macOS ARM64 is the
verified runtime; cross-compilation is not certification. Unix checks have
process-group supervision; non-Unix checks refuse launch pending equivalent
descendant cleanup.

Commands target the current directory unless `--root PATH` appears before the
command. The executable's location does not select a root. Relocation requires
a compatible separately installed binary, not the original checkout.
Template content comes from that binary, never from local workspace overrides.
Do not edit generated output as product source; use a trusted source checkout
for product changes. Binary replacement and managed-content updates are separate.

## Command reference

| Command | Options and result |
| --- | --- |
| `validate` | Validates `wsup.json`, settings, and adapter links without executing tasks; prints a summary. |
| `projects` | Prints project entries as JSON. |
| `config` | Prints effective settings and their sources as JSON. |
| `guidance` | Prints structural findings as JSON; optional `--project ID` selects a registered owner. |
| `vscode` | Previews regenerating the `vscode` adapter's `<name>.code-workspace` from `wsup.json`; `--apply` writes it unless hand-edited; other files are left to `update`. |
| `run PROJECT TASK` | Runs any one task with the terminal's input and output and exits with its exit code (124 on timeout, 130 when interrupted); only the task's `timeout_seconds` or `--timeout` limits it; no report. |
| `check` | Runs check tasks: repeatable `--project ID` or `--all`; optional repeatable `--task NAME`, `--timeout SECONDS`, `--show-output`. |
| `init DESTINATION` | Creates a new workspace. In a terminal it asks for the preset, agents, and editors that flags leave open; `--yes`, or no terminal, uses the defaults; repeatable `--agent NAME` and `--editor NAME` select agent and editor adapters (default all; `none` for none; editors are workspace-only); `--preset NAME` selects a preset (default `development`; `none` for the base only) and repeatable `--collection NAME` adds collections to it. `--template project` creates a project without a registry/runtime; presets and collections are workspace-only. |
| `update` | Prints a managed-content plan; `--apply` writes when conflict-free, repeatable `--keep PATH` retains eligible same-path customizations. |
| `migrate` | Previews converting a wsup 0.2.0 workspace or project; `--apply` converts it when conflict-free; `--agent` and `--editor` as for `init`; repeatable `--keep PATH` keeps an owner file at a path the conversion installs. |
| `add MODULE...` | Previews adding collections or agent/editor adapters, with the modules they require; `--apply` writes when conflict-free; repeatable `--keep PATH` keeps an owner file at a path the addition installs. |
| `modules [ID]` | Lists built-in modules as JSON (`--kind KIND` filters); with an ID, shows its manifest and files. |
| `schema` | Prints the JSON Schema for `wsup.json` (the file `$schema` names). |

Use `--help`, `COMMAND --help`, and `--version` for options and product version.
Invalid options fail even when help/version is also present. Unambiguous long
option prefixes are supported. Normal operational errors print handled stderr.

Initialization requires a new destination, including when an existing directory
is empty. Relative destinations use the current process directory, not `--root`.
No registry is required to run `init`. All files are exclusively created,
`wsup.lock` is written last, and existing destinations are never replaced. A write failure
may leave partial output; it is reported and retained for inspection, not
recursively removed. No transactional rollback or power-loss durability is promised.

Exit codes are 0 for success (including report-only findings), 1 for guidance
findings or unsuccessful required checks, 2 for invalid input/operational errors,
and 130 for interrupted checks. Finding exit codes are per-check policy, not
CLI exit codes.

## Default layout and optional collections

| Home | Responsibility |
| --- | --- |
| `apps/`\* | Applications and user-facing deliverables |
| `packages/`\* | Reusable libraries/packages |
| `tools/`\* | Local developer tools |
| `infra/`\* | Provisioning, deployment, and operations code |
| `docs/` | Maintained workspace documentation |
| `workbench/` | Shareable drafts and temporary analysis |
| `AGENTS.md`, `.agents/skills/` | Agent instructions and portable procedures |
| `wsup.json` | Workspace name, selected modules, settings, and projects (owner-edited) |
| `wsup.lock` | Installed modules and managed-file hashes (written by wsup) |

\* Collections selected by the default `development` preset. `--preset none`
omits them; they can be added later like any other collection.

Homes organize responsibility, not repository topology or required population.
Project `kind` is independent of directory name.
Local session plans/notes live in ignored `workbench/.local/`, created on demand;
check reports go to `workbench/.local/wsup/reports/`. Review and sanitize drafts
before staging.
Promote accepted knowledge to its maintained owner rather than keeping competing
canonical copies. A journal is optional.

Every other collection is created only when selected:

| Group | Collections |
| --- | --- |
| Projects | `apps`, `packages`, `tools`, `infra`, `sandbox`, `templates`, `forks` |
| Knowledge and lifecycle | `research`, `reference`, `decisions`, `learning`, `journal`, `archive`, `releases` |
| Work and making | `issues`, `ai`, `design`, `content`, `data` |

These are organizational
choices, not permissions, encryption, sync, backup, or retention controls.
Restricted material belongs outside documentation and Git unless appropriate
controls are separately established. Ignored does not mean confidential.

```sh
wsup init /path/to/new-workspace --collection archive --collection journal
wsup init /path/to/new-workspace --preset none --collection research
wsup modules --kind collection
wsup add decisions issues
wsup add decisions issues --apply
```

Preset and collection names are validated, sorted, and deduplicated before
anything is written. The binary carries every collection source; only selected
ones are installed and managed. A child workspace starts from its own preset
and selections; it never inherits the parent's collections.
`wsup modules` needs no workspace. Additions require `wsup.lock`, and in a workspace `wsup.json`; a project can add agent adapters. Required modules are listed under `installs` in the preview, never added silently.
Unknown names, project selections, and symlink destinations fail explicitly.

Unmanaged existing scaffold files are conflicts even when identical; they are
never adopted or overwritten. `wsup add --keep PATH` keeps one as your
customization instead, as `migrate --keep` does. Missing selected managed files are conflicts.
Already-selected regular managed files, including customizations, are left
unchanged with their original hashes; use update to review those customizations.
Unselected managed collection files conflict rather than being implicitly adopted.
All conflicts block additions. An addition also adds the collections to
`wsup.json`, which is rewritten in its canonical form. `wsup.lock` and observed
paths are rechecked before exclusive writes; the lock is replaced last,
preserving unrelated hashes and previous modules. I/O failures can leave new
files with the old lock and are explicit; inspect partial results before retrying.
These commands configure no permissions, ACLs, sync, backups, or retention.
Selected modules' ignore entries are written to the `.gitignore` block
between `# wsup:begin ignore` and `# wsup:end ignore`: `data` ignores
`data/**/*.local.*`, and the `claude` adapter ignores Claude Code's per-user
`.claude/settings.local.json` (in projects too). Files rendered from the
selection, such as that `.gitignore`, are rewritten while they match their
baseline; an edited copy gets only its generated block replaced (`merge`) when
that block is unchanged, and is otherwise a conflict.

## Module catalogue

`wsup modules` lists every built-in module and `wsup modules ID` shows one
module's manifest and files. Every collection installs `<folder>/README.md`
and `<folder>/AGENTS.md`; the table lists what else it adds. The code-change
skills are make-change, review-change, diagnose-issue, write-commit, and
infer-conventions.

| Collection | Holds | Extra files | Ignores | Skills |
| --- | --- | --- | --- | --- |
| `apps` | Applications and user-facing deliverables | — | — | code-change |
| `packages` | Reusable libraries and packages | — | — | code-change |
| `tools` | Local developer tools | — | — | code-change |
| `infra` | Provisioning, deployment, and operations code | — | — | code-change |
| `sandbox` | Experiments, spikes, and throwaway prototypes | — | — | code-change |
| `templates` | Project starters and scaffolds other projects are created from | — | — | code-change |
| `forks` | Forks and vendored copies of external repositories | — | — | code-change |
| `research` | Scoped workspace-wide investigations | — | — | — |
| `reference` | Reviewed, reusable knowledge with evidence and applicability | — | — | — |
| `decisions` | Workspace-wide decision records | `TEMPLATE.md` | — | — |
| `learning` | Curricula, exercises, and learning projects | — | — | — |
| `journal` | Dated activity entries | `TEMPLATE.md` | — | — |
| `archive` | Parked or historical work with a clear owner and status | — | — | — |
| `releases` | Intentional deliverables with provenance and retention purpose | — | — | — |
| `issues` | Keyed Markdown issue files | `TEMPLATE.md` | — | — |
| `ai` | Prompts, evaluations, and other reusable AI working material | — | — | — |
| `design` | Mockups, design-system assets, and design sources | — | — | — |
| `content` | Written and media content meant for publication | — | — | — |
| `data` | Datasets, samples, and fixtures shared across projects | — | `data/**/*.local.*` | — |

The bases install the skills every starter has:

| Base | Skills |
| --- | --- |
| `workspace-base` | check-workspace, run-checks, update-workspace, onboard-project, plan-work, recap-work, maintain-agents-md |
| `project-base` | make-change, review-change, diagnose-issue, write-commit, infer-conventions, plan-work |

A skill listed by several selected modules is installed once. Adapters are
described under [Guidance](#agent-adapters-and-the-skills-link).

### Presets

A preset selects collections at `init` and nowhere else. `wsup.json` records
the resolved collections, so a later change to a preset never alters an
existing workspace.

| Preset | Collections |
| --- | --- |
| `development` (default) | `apps`, `packages`, `tools`, `infra` |
| `none` | The base only |

`--collection NAME` adds to the preset: `--preset none --collection research`
creates the base plus `research/`.

## Register projects

`wsup.json` is UTF-8 JSON with `version: 3`, a nonempty `name`, and a `projects`
array. Its `$schema` lets editors validate and complete it; `wsup schema` prints
the same schema. `init` writes it with `$schema`, the selected `collections`, `agents`, and
`editors`, and no projects. Optional `settings` hold shared settings (see below).
Optional `vscode` holds `settings`, `extensions`, `tasks`, and `launch` for VS
Code. Projects have:

| Field | Contract |
| --- | --- |
| `id` | Unique ID beginning with a letter/digit, followed by letters/digits, `.`, `_`, or `-` |
| `path` | Unique relative forward-slash path to an existing directory with README |
| `kind` | `app`, `package`, `tool`, `infra`, `sandbox`, `template`, `fork`, or `research` |
| `repository` | `workspace` or `independent` (intent, not proof of Git setup) |
| `git` | Optional clone URL |
| `purpose`, `owner` | Nonempty descriptions |
| `dependencies` | Optional IDs of other projects, resolving without cycles |
| `tasks` | Optional named commands (see below) |

Paths must stay within the workspace, including through symlinks. Unknown
fields are rejected. The registry does not clone, initialize Git, install
dependencies, or create a shared release. Create a project first, then edit its
entry:

```json
{
  "version": 3,
  "name": "Example",
  "projects": [
    {
      "id": "app",
      "path": "apps/app",
      "kind": "app",
      "repository": "workspace",
      "purpose": "Example application",
      "owner": "Application maintainers",
      "tasks": {
        "test": {"command": ["go", "test", "./..."], "check": true}
      }
    }
  ]
}
```

Run `validate` and `projects` after editing. There is no project-add command.
`wsup.json` belongs to the workspace: `update` reads it but never rewrites it,
and `wsup add` only adds to its `collections`, `agents`, and `editors` lists.

## Git ownership

Keep `wsup.json`, `wsup.lock`, grouping READMEs, guidance, and workspace-owned
projects in the workspace repository. Independent projects have their own
repositories; register them without implying shared builds or releases.

For an independent project:

1. Ignore its exact root-relative directory, e.g. `/apps/my-app/`, in this
   workspace. Do not ignore a whole grouping directory or only nested `.git`.
2. Clone/create its repository only when requested. Verify the project's Git
   top-level is the project itself.
3. Register its path and `"repository": "independent"`.
4. Check `git check-ignore -v apps/my-app/` and workspace/project status before
   staging. Commit independent project changes from their own repository.

Metadata and ignore rules do not untrack already-tracked content. Preserve work
and history, get approval, and deliberately remove paths from the workspace index
when converting ownership. Never force-add independent projects or create
submodules implicitly.

The starter ignores `wsup.local.json`, `.env`/`.env.*` except `.env.example`,
`.DS_Store`, and `workbench/.local/` (which holds reports), plus the selected
collections' entries in its marked block. `.editorconfig` (UTF-8, LF, final
newline, trimmed trailing whitespace) and `.gitattributes` (`* text=auto eol=lf`)
are owner-editable starter files.
Every repository owns its ignore rules; these are staging hygiene, not security.
Do not ignore `wsup.lock`.

## Tasks and checks

A task is a literal argv array or an object with `command` and optional
`check`, `cwd`, `mode`, `enabled`, `reason`, `timeout_seconds`, and
`finding_exit_codes`. A task with `"check": true` runs under `wsup check`:

```json
"tasks": {
  "test": {"command": ["go", "test", "./..."], "check": true},
  "audit": {"command": ["my-audit-tool"], "check": true, "mode": "report"},
  "build": ["go", "build", "./..."]
}
```

Names follow the project-ID format. Executables must be installed and supplied
by explicit path or found on PATH; there is no tool fallback or installation.
Arguments are never substituted or shell-expanded, stdin is closed, and cwd is
the registered project directory, or `cwd` relative to it. Inspect commands
first: they execute with your permissions and are not sandboxed.

Required mode fails on nonzero exit; report mode treats configured finding
exit codes (default `[1]`) as findings, not CLI failure. Empty finding codes mean
no advisory exits. Genuine failures, missing tools, timeout, and required skips
still fail. Disabled checks need a nonempty reason. Dependencies are descriptive;
only explicitly selected projects run. With no `--task`, every check task runs;
each requested task must exist and be a check task on every selected project.
`--all` selects every project with matching check tasks. Selection is fully
validated before launching tools or creating reports.

`wsup run PROJECT TASK` runs any task, check or not, the same literal way but
connected to the terminal. Interrupting it asks the task's process group to
stop and kills it after five seconds.

```sh
wsup run app build
wsup check --project app --project library --task test --show-output
wsup check --all
wsup check --project app --timeout 60
```

Ordinary failures do not stop later checks. Ctrl-C records cancellation, skips
remaining checks, and returns 130. Timeout/cancellation terminates the Unix
process group. Descendants deliberately escaping that group are not guaranteed
to be killed; inherited-pipe cleanup is bounded and cleanup failures explicit.
Output is suppressed by default. `--show-output` prints bounded captured output
after each check, not live output, and can reveal secrets in the terminal.

## Settings and reports

Configuration precedence is defaults, `settings` in `wsup.json`, ignored
`wsup.local.json`, environment, then explicit CLI overrides. `wsup.local.json`
uses `{"version": 1, "settings": {...}}`; unknown fields/invalid values fail.

| Setting | Default and bounds | Environment |
| --- | --- | --- |
| `timeout_seconds` | 300; 1..86400 | `WSUP_TIMEOUT_SECONDS` |
| `output_limit_bytes` | 65536 per stream; 1..10485760 | `WSUP_OUTPUT_LIMIT_BYTES` |
| `report_dir` | `workbench/.local/wsup/reports`; must remain inside it | `WSUP_REPORT_DIR` |

Check timeout overrides workspace timeout; CLI `--timeout` overrides both.
`config` reports values and source labels. Settings are local to this workspace,
never inherited. A symlink anywhere in `workbench/.local/wsup/reports` is
refused; contained nested report paths may be used.

Checks write JSON/Markdown schema-v2 reports with Go/OS/architecture metadata,
selection and original-command SHA-256 hashes, current input-byte hashes,
sanitized execution metrics/statuses, and optional Git state. They do not store
command arguments, captured output, environment values, or credentials.
Project IDs, names, paths, and Git state can still be sensitive: reports are
ignored local evidence, not automatically safe to publish.

Files are created exclusively, mode 0600 on POSIX. Windows mode bits do not
establish equivalent ACLs. Collisions do not overwrite files. A later failure may
leave a partial pair and returns an explicit error; reports are not transactional.
No existing report is rewritten to change provenance.

## Generated files and updates

`wsup.lock` (version 1) records the wsup version, the starter (`workspace` or
`project`), each installed module and its version, and baseline SHA-256 hashes
of managed files, of each generated section, and of the skills link. It never
lists `wsup.json` or reports. Commit it with the files so future updates can
identify customizations. wsup writes it; do not edit it by hand.

```json
{
  "version": 1,
  "wsup_version": "0.3.0",
  "starter": "project",
  "modules": {"claude": "1.1.0", "project-base": "1.0.0"},
  "files": {"AGENTS.md": "sha256:…", "CLAUDE.md": "sha256:…"},
  "blocks": {"AGENTS.md#skills": "sha256:…", ".gitignore#ignore": "sha256:…"},
  "links": {".claude/skills": "../.agents/skills"}
}
```

| Field | Contract |
| --- | --- |
| `version` | Lock format; only `1` is supported |
| `wsup_version` | The wsup version that last wrote the lock |
| `starter` | `workspace` or `project` |
| `modules` | Every installed module (bases, collections, adapters; never presets) and its version |
| `files` | Each managed file's baseline hash |
| `blocks` | Each generated section's baseline hash, as `FILE#NAME` |
| `links` | The skills link and its target |
Local instructions and skills are workspace customizations, not implicit defaults for child
generation. A new workspace always uses the binary's embedded baseline.

`wsup update` compares managed files with the invoked binary's embedded
templates for the modules in `wsup.lock`; generators read the current
`wsup.json`. Preview is read-only. `wsup update --apply` blocks on all
conflicts, rechecks the lock and observed files, creates new files exclusively,
replaces existing files without in-place truncation, and writes the next lock last. Edited or missing managed files require manual resolution,
except an edited file whose generated sections are unchanged, which is merged
(see Guidance). `--keep PATH` can retain an existing same-path customization without accepting
its current hash as a new baseline. Repeat it on later updates if still needed.
The preview's `modules` report lists installed modules whose version in this
wsup differs (`updated`; applying records the new version), ids `wsup.json`
lists that are not installed (`offered`; install them with `wsup add`, never
silently), and installed modules `wsup.json` no longer lists (`unlisted`;
nothing is removed). `wsup.json` remains workspace-owned and is never
regenerated by update.
Orphan bytes are retained but cease to be managed, except a generated file
replaced under a new name, which is removed while unedited. Unknown files,
project contents, reports, and local settings survive.

Only lock version 1 is supported. Other versions fail without writes. Do not
change version numbers or fabricate baselines to force compatibility.

## Migrating from wsup 0.2.0

A workspace or project created by wsup 0.2.0 has `.wsup/template-state.json`
instead of `wsup.lock`; commands that read `wsup.json` or `wsup.lock` report
that it needs converting.
`wsup migrate` previews the conversion and `wsup migrate --apply` performs it:

- Files wsup wrote and you never edited are replaced with their current
  version, or removed when wsup no longer installs them (`AI.md`,
  `guidance/rules/`, renamed skills, `WSUP-LICENSE`, `.gitkeep` homes).
- Files you edited are kept (`keep`). One wsup still installs, such as
  `README.md`, keeps its 0.2.0 baseline, so `update` reports it until you
  `--keep` it or replace it. One wsup no longer installs is noted, for example
  to fold an edited `AI.md` into `AGENTS.md`.
- `workspace.json` and the settings-only `wsup.json` become `wsup.json`:
  checks become tasks with `"check": true`, the `infrastructure` kind becomes
  `infra`, and a `report_dir` under `.wsup/reports` moves under
  `workbench/.local/wsup/reports` (in `wsup.local.json` too).
- `.wsup/reports/` moves to `workbench/.local/wsup/reports/`; `.wsup/` is
  removed.
- Former default homes it still manages become collections (`infrastructure/`
  selects `infra`); `artifacts` becomes `releases` and `private` is dropped.
  Folders holding your content are never moved; the preview's notes say where
  they belong.
- Agent and editor adapters default to all, as at `init`.

An existing unmanaged file at a path the conversion installs (such as an
owner-written `CLAUDE.md` or `apps/README.md`) is a conflict, and any conflict
blocks every write. Repeat `--keep PATH` to keep such a file instead: it is
left as it is and recorded against wsup's version, so `update` reports it
until `update --keep` accepts it. A `--keep` path that names no such file fails.
Before writing, every file to be replaced or removed is copied to
`workbench/.local/wsup/migrate/<timestamp>/`; `wsup.lock` is written after the
content and the 0.2.0 state is removed last. If a migration fails partway,
restore from that folder before retrying.
No remote downloader or automatic binary upgrade is provided.

Before any later update, preserve a reviewed backup of files and `wsup.lock`,
inspect Git status and the complete preview, and resolve conflicts deliberately.
Native updates are not a multi-file transaction; I/O failure
can leave partial content with the old lock. Concurrent mutation after rechecks is
not isolated, and no fsync/power-loss durability guarantee is made. Inspect
partial destinations rather than deleting owner content blindly. Recovery means
restoring matched files and `wsup.lock` from the same backup and using a compatible
binary. No commit, installation, or publication is automatic.

## Guidance

Read the root `AGENTS.md` and the AGENTS.md of the folder you work in. The root
file's Layout and Skills sections sit between `<!-- wsup:begin NAME -->` /
`<!-- wsup:end NAME -->` markers and are generated from the selection; text
outside the markers belongs to the owner. When `update` or a collection
addition changes an edited AGENTS.md or `.gitignore`, it keeps the owner text
and replaces only the generated sections (the `merge` action), provided the
owner left those sections as `wsup.lock` recorded them; otherwise the file is a
conflict. Seven base skills are always
included under `.agents/skills/`; project collections (`apps`, `packages`,
`tools`, `infra`, `sandbox`, `templates`, `forks`) add five code-change skills,
installed once however many are selected. Adding a collection installs any of
its skills not already present. No provider, credential, or MCP server is
required. Clients may discover that shared path or need explicit skill
paths. Verify client loading separately from file presence. `guidance` checks
metadata, local links, marker pairing, owner containment, and structural findings, not whether
an AI client invoked a skill. Findings are sorted lexically, preserving duplicates.
Independent projects keep their required guidance themselves: `guidance`
reads `AGENTS.md` one folder deep only, and skips a folder that has its own
`wsup.lock` (a nested workspace or project).

### Agent adapters and the skills link

An agent adapter only adds pointers for an agent that does not read `AGENTS.md`
or `.agents/skills/` natively; it never holds instructions. `init` installs every
adapter unless `--agent NAME` (repeatable) or `--agent none` says otherwise,
for workspaces and projects alike. The installed adapters are recorded in
`wsup.json` and `wsup.lock`; `update` keeps them current but never installs an adapter that
was not selected, including adapters added in a later wsup version.

| Adapter | Files |
| --- | --- |
| `claude` | `CLAUDE.md` (`@AGENTS.md`), `.claude/skills -> ../.agents/skills` |
| `gemini` | `GEMINI.md` (`@AGENTS.md`) |

`.claude/skills` is the only symlink wsup creates, and a module may declare no
other link: its target must be the relative path to `.agents/skills`. It is
recorded in `wsup.lock` as a link with its target, not as a file hash. `update`
re-creates it when missing and reports a conflict, leaving the path alone, when
it is a real directory or points elsewhere. `validate` and `guidance` report a
missing, wrong, or broken link. The relative target keeps working when the
workspace is moved. Every other wsup read still refuses to follow links out of
the workspace root.

### Editor adapters

`vscode` (installed by default for workspaces; `--editor none` skips it)
generates `<name>.code-workspace`, named from the slugified `name` in
`wsup.json` (for example `my-workspace.code-workspace`). It covers VS Code,
Cursor, Windsurf, and VSCodium. The file lists the workspace root and one folder
per registered project, copies the `vscode` block's `settings`, `extensions`
(as recommendations), and `launch` configurations, and has one VS Code task per
project task (labelled `<project>: <task>`, check tasks in the test group)
followed by the `vscode` block's own `tasks`. It is a managed starter file: `update` regenerates it
from the current `wsup.json` and writes it while it is unedited, reports a
hand-edited copy as a conflict, and stops if `wsup.json` cannot be read. `wsup vscode` does the same for that file alone.
Renaming the workspace writes the new file and removes the old one while it is
unedited; an edited old file is a conflict. There is
no adapter for JetBrains, Zed, or terminal editors; every editor benefits from
`.editorconfig`.
