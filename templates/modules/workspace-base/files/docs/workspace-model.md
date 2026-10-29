# Workspace model

## Installation and ownership

Each workspace knows only itself and its descendants. Its registry, guidance,
documentation, and commands describe its own root. No parent discovery,
configuration inheritance, or relationship to the generating workspace exists.
Descendant projects/workspaces retain self-contained instructions.

Use a separately installed native `wsup` executable with embedded templates.
The workspace contains managed files and `.wsup/template-state.json`, not a CLI
runtime, binary, or copied catalogue. Reports are under `.wsup/reports/`.
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
| `validate` | Validates registry/settings without executing checks; prints a summary. |
| `projects` | Prints project entries as JSON. |
| `config` | Prints effective settings and their sources as JSON. |
| `guidance` | Prints structural findings as JSON; optional `--project ID` selects a registered owner. |
| `editor` | Creates a relative-path VS Code workspace file, identical to the `vscode` adapter's; `--output NAME` must be a new file directly in the root. |
| `check` | Requires repeatable `--project ID`; optional repeatable `--check NAME`, `--timeout SECONDS`, `--show-output`. |
| `init DESTINATION` | Creates a new workspace. In a terminal it asks for the preset, agents, and editors that flags leave open; `--yes`, or no terminal, uses the defaults; repeatable `--agent NAME` and `--editor NAME` select agent and editor adapters (default all; `none` for none; editors are workspace-only); `--preset NAME` selects a preset (default `development`; `none` for the base only) and repeatable `--collection NAME` adds collections to it. `--template project` creates a project without a registry/runtime; presets and collections are workspace-only. |
| `update` | Prints a managed-content plan; `--apply` writes when conflict-free, repeatable `--keep PATH` retains eligible same-path customizations. |
| `collections` | Lists available names; repeatable `--add NAME` previews additions, `--apply` writes when conflict-free. |

Use `--help`, `COMMAND --help`, and `--version` for options and product version.
Invalid options fail even when help/version is also present. Unambiguous long
option prefixes are supported. Normal operational errors print handled stderr.

Initialization requires a new destination, including when an existing directory
is empty. Relative destinations use the current process directory, not `--root`.
No registry is required to run `init`. All files are exclusively created, state
is written last, and existing destinations are never replaced. A write failure
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
| `.wsup/` | Template state and ignored check reports |

\* Collections selected by the default `development` preset. `--preset none`
omits them; they can be added later like any other collection.

Homes organize responsibility, not repository topology or required population.
Project `kind` is independent of directory name.
Local session plans/notes live in ignored `workbench/.local/`, created on demand;
legacy `.work/` exclusions remain for compatibility. Historical reports stay
under `.wsup/reports/`, not workbench. Review and sanitize drafts before staging.
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
wsup collections
wsup collections --add decisions --add issues
wsup collections --add decisions --add issues --apply
```

Preset and collection names are validated, sorted, and deduplicated before
anything is written. The binary carries every collection source; only selected
ones are installed and managed. A child workspace starts from its own preset
and selections; it never inherits the parent's collections.
Listing needs no managed state; additions require a native managed workspace.
Unknown names, project selections, and symlink destinations fail explicitly.

Unmanaged existing scaffold files are conflicts even when identical; they are
never adopted or overwritten. Missing selected managed files are conflicts.
Already-selected regular managed files, including customizations, are left
unchanged with their original hashes; use update to review those customizations.
Former default-managed journal/learning/reference/research files can be explicitly
selected without accepting customized bytes as baselines. Workspaces created when
`apps/`, `packages/`, and `tools/` were part of the base keep them: a managed file
there selects its collection, while an omitted one leaves it unselected. A former
`infrastructure/` home is retained but no longer managed; renamed (`artifacts`)
or removed (`private`) selections fail until migrated. Other unselected
managed collection files conflict rather than being implicitly adopted.
All conflicts block additions. State and observed paths are rechecked before
exclusive writes; state is replaced last, preserving unrelated hashes, template
version, omitted homes, and previous selections. I/O failures can leave new files
with old state and are explicit; inspect partial results before retrying.
These commands configure no permissions, ACLs, sync, backups, or retention.
Selected collections' ignore entries are written to the `.gitignore` block
between `# wsup:begin ignore` and `# wsup:end ignore` (today only `data`, which
ignores `data/**/*.local.*`). Files rendered from the selection, such as that
`.gitignore`, are rewritten by an addition only while they match their baseline;
an edited copy is a conflict.

## Register projects

`workspace.json` is UTF-8 JSON with `version: 1`, nonempty `name`, and a `projects`
array. Projects have:

| Field | Contract |
| --- | --- |
| `id` | Unique ID beginning with a letter/digit, followed by letters/digits, `.`, `_`, or `-` |
| `path` | Unique relative forward-slash path to an existing directory with README |
| `kind` | `app`, `package`, `tool`, `infrastructure`, `research`, or `template` |
| `repository` | `workspace` or `independent` (intent, not proof of Git setup) |
| `purpose`, `owner` | Nonempty descriptions |
| `dependencies` | Optional IDs of other projects, resolving without cycles |
| `checks` | Optional named check definitions |

Paths must stay within the workspace, including through symlinks. Unknown
registry/project fields are rejected. The registry does not clone, initialize
Git, install dependencies, or create a shared release. Create a project first,
then edit its registry entry:

```json
{
  "version": 1,
  "name": "Example",
  "projects": [
    {
      "id": "app",
      "path": "apps/app",
      "kind": "app",
      "repository": "workspace",
      "purpose": "Example application",
      "owner": "Application maintainers"
    }
  ]
}
```

Run `validate` and `projects` after editing. There is no project-add command.

## Git ownership

Keep registry, configuration, grouping READMEs, state, guidance, and workspace-owned
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

The starter ignores `.wsup/reports/`, `wsup.local.json`, `.env`/`.env.*` except
`.env.example`, `.DS_Store`, `workbench/.local/`, and `.work/`, plus the selected
collections' entries in its marked block. `.editorconfig` (UTF-8, LF, final
newline, trimmed trailing whitespace) and `.gitattributes` (`* text=auto eol=lf`)
are owner-editable starter files.
Every repository owns its ignore rules; these are staging hygiene, not security.
Do not ignore template state or all of `.wsup/`.

## Configure checks

Checks are literal argv arrays or objects with `command` and optional `mode`,
`enabled`, `reason`, `timeout_seconds`, and `finding_exit_codes`:

```json
"checks": {
  "test": ["go", "test", "./..."],
  "audit": {"command": ["my-audit-tool"], "mode": "report"}
}
```

Names follow the project-ID format. Executables must be installed and supplied
by explicit path or found on PATH; there is no tool fallback or installation.
Arguments are never substituted or shell-expanded, stdin is
closed, and cwd is the registered project directory. Inspect commands first:
they execute with your permissions and are not sandboxed.

Required mode fails on nonzero exit; report mode treats configured finding
exit codes (default `[1]`) as findings, not CLI failure. Empty finding codes mean
no advisory exits. Genuine failures, missing tools, timeout, and required skips
still fail. Disabled checks need a nonempty reason. Dependencies are descriptive;
only explicitly selected projects/checks run. With no `--check`, all configured
checks run; each requested check name must exist on every selected project.
Selection is fully validated before launching tools or creating reports.

```sh
wsup check --project app --project library --check test --show-output
wsup check --project app --timeout 60
```

Ordinary failures do not stop later checks. Ctrl-C records cancellation, skips
remaining checks, and returns 130. Timeout/cancellation terminates the Unix
process group. Descendants deliberately escaping that group are not guaranteed
to be killed; inherited-pipe cleanup is bounded and cleanup failures explicit.
Output is suppressed by default. `--show-output` prints bounded captured output
after each check, not live output, and can reveal secrets in the terminal.

## Settings and reports

Configuration precedence is defaults, shared `wsup.json`, ignored
`wsup.local.json`, environment, then explicit CLI overrides. Configuration files
use `{"version": 1, "settings": {...}}`; unknown fields/invalid values fail.

| Setting | Default and bounds | Environment |
| --- | --- | --- |
| `timeout_seconds` | 300; 1..86400 | `WSUP_TIMEOUT_SECONDS` |
| `output_limit_bytes` | 65536 per stream; 1..10485760 | `WSUP_OUTPUT_LIMIT_BYTES` |
| `report_dir` | `.wsup/reports`; must remain inside it | `WSUP_REPORT_DIR` |

Check timeout overrides workspace timeout; CLI `--timeout` overrides both.
`config` reports values and source labels. Settings are local to this workspace,
never inherited. Symlinked `.wsup/` or reports roots are refused; contained nested
report paths may be used. Do not use runtime/catalogue paths as reports.

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

Native schema-2 template state stores baseline SHA-256 hashes and product/template
version. It owns managed starter files, never runtime/report content under
`.wsup/`. Keep state with the files so future updates can identify customizations.
Local instructions and skills are workspace customizations, not implicit defaults for child
generation. A new workspace always uses the binary's embedded baseline.

`wsup update` compares managed files with the invoked binary's embedded template
and recorded collection selections. Preview is read-only. `wsup update --apply`
blocks on all conflicts, rechecks observed state/files, creates new files
exclusively, replaces existing files without in-place truncation, and writes
next state last. Edited or missing managed files require manual resolution;
`--keep PATH` can retain an existing same-path customization without accepting
its current hash as a new baseline. Repeat it on later updates if still needed.
The registry remains workspace-owned and is never regenerated by update.
Omitted default homes remain omitted; orphan bytes are retained but cease to be
managed, except a generated file replaced under a new name, which is removed
while unedited. Unknown files, project contents, reports, and local settings survive.
There are no runtime files to update or retire in native schema 2.

Only schema-2 state is supported. Other versions fail without writes; no
in-place conversion or obsolete installation cleanup is provided. Do not change
schema numbers or fabricate baselines to force compatibility. For an unsupported
workspace, initialize a new destination and explicitly transfer owner content
after review; preserve the original workspace and its reports.
No remote downloader or automatic binary upgrade is provided.

Before any later update, preserve a reviewed backup of files and state,
inspect Git status and the complete preview, and resolve conflicts deliberately.
Native updates are not a multi-file transaction; I/O failure
can leave partial content with old state. Concurrent mutation after rechecks is
not isolated, and no fsync/power-loss durability guarantee is made. Inspect
partial destinations rather than deleting owner content blindly. Recovery means
restoring matched files and state from the same backup and using a compatible
binary. No commit, installation, or publication is automatic.

## Guidance

Read the root `AGENTS.md` and the AGENTS.md of the folder you work in. The root
file's Layout and Skills sections sit between `<!-- wsup:begin NAME -->` /
`<!-- wsup:end NAME -->` markers and are generated from the selection; text
outside the markers belongs to the owner. When a collection is added, an edited
AGENTS.md or `.gitignore` keeps its owner text and only its generated sections
are replaced, provided the owner left those sections unchanged; otherwise the
file is a conflict. `update` still compares whole files. Seven base skills are always
included under `.agents/skills/`; project collections (`apps`, `packages`,
`tools`, `infra`, `sandbox`, `templates`, `forks`) add five code-change skills,
installed once however many are selected. Adding a collection installs any of
its skills not already present. No provider, credential, or MCP server is
required. Clients may discover that shared path or need explicit skill
paths. Verify client loading separately from file presence. `guidance` checks
metadata, local links, marker pairing, owner containment, and structural findings, not whether
an AI client invoked a skill. Findings are sorted lexically, preserving duplicates.
Independent projects keep their required guidance themselves.

### Agent adapters and the skills link

An agent adapter only adds pointers for an agent that does not read `AGENTS.md`
or `.agents/skills/` natively; it never holds instructions. `init` installs every
adapter unless `--agent NAME` (repeatable) or `--agent none` says otherwise,
for workspaces and projects alike. The installed adapters are recorded in
template state; `update` keeps them current but never installs an adapter that
was not selected, including adapters added in a later wsup version.

| Adapter | Files |
| --- | --- |
| `claude` | `CLAUDE.md` (`@AGENTS.md`), `.claude/skills -> ../.agents/skills` |
| `gemini` | `GEMINI.md` (`@AGENTS.md`) |

`.claude/skills` is the only symlink wsup creates, and a module may declare no
other link: its target must be the relative path to `.agents/skills`. It is
recorded in state as a link with its target, not as a file hash. `update`
re-creates it when missing and reports a conflict, leaving the path alone, when
it is a real directory or points elsewhere. `validate` and `guidance` report a
missing, wrong, or broken link. The relative target keeps working when the
workspace is moved. Every other wsup read still refuses to follow links out of
the workspace root.

### Editor adapters

`vscode` (installed by default for workspaces; `--editor none` skips it)
generates `<name>.code-workspace`, named from the slugified `name` in
`workspace.json` (for example `my-workspace.code-workspace`). It covers VS Code,
Cursor, Windsurf, and VSCodium. The file lists the workspace root and one folder
per registered project. It is a managed starter file: `update` regenerates it
from the current registry and writes it while it is unedited, reports a
hand-edited copy as a conflict, and stops if the registry cannot be read.
Renaming the workspace writes the new file and removes the old one while it is
unedited; an edited old file is a conflict. There is
no adapter for JetBrains, Zed, or terminal editors; every editor benefits from
`.editorconfig`.
