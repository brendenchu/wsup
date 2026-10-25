# Workspace model

The directory layout groups work by responsibility. `workspace.json` records
project ownership and local checks. Git topology is independent: one workspace
may contain a single repository, independent repositories, or a mixture.
Directories are optional, and no shared build or release is implied.
New workspaces omit empty project grouping directories and journals. Create a
collection when it has real contents. Project `kind` classifies purpose; it does
not require a matching directory name or mirror a personal folder taxonomy.

## Installation and ownership

Generated workspaces run `python3 -B .wsup/wsup.py ...` from the workspace root.
The development checkout instead runs `tools/wsup.py`; its visible `tools/` and
`templates/` are product source, not an installed consumer.
Commands target the current directory unless `--root PATH` is supplied.

The generated installation contains:

```text
.wsup/
  wsup.py
  wsup_core/
  templates/
  template-state.json
  reports/
```

The installed `.wsup/templates/` is the single generation catalogue, not a
second user-customization surface. Change product starters in a trusted source
checkout, then preview updates before refreshing. Local edits become conflicts.
Generated workspaces can generate another workspace without that checkout.
Local workspace rules and skills are not implicit overrides for newly generated
workspaces; initialization reads the installed catalogue. In the source checkout,
root `.agents/skills/` remains the canonical development skill source.

Keep state, runtime, and bundled templates eligible for local tracking; ignore
only generated reports. The registry, `AI.md`, `guidance/rules/`, and
`.agents/skills/` remain workspace-owned public surfaces. `AGENTS.md` points to
`AI.md`. Human drafts can live in visible `workbench/`; decide their tracking
separately from sensitivity and publication.

## Register projects

`workspace.json` is UTF-8 JSON with `version: 1`, a nonempty `name`, and a
`projects` array. Each project has:

| Field | Meaning |
| --- | --- |
| `id` | Unique identifier using letters, digits, `.`, `_` or `-` |
| `path` | Existing workspace-relative directory containing `README.md` |
| `kind` | Coarse purpose (`app`, `package`, `tool`, `infrastructure`, `research`, `template`); independent of folder path |
| `repository` | `workspace` or `independent`, recording intended ownership |
| `purpose`, `owner` | Nonempty project description and responsible owner |
| `dependencies` | Optional array of other project IDs |
| `checks` | Optional named check definitions |

Paths use relative forward slashes and must remain inside the workspace, including
through symlinks. Project paths must be unique. Dependencies must resolve and be
acyclic. The registry describes intent; it does not create or verify Git
repositories, clone projects, install dependencies, or release packages.

## Git ownership

Keep workspace configuration, guidance, and any grouping-directory README files
in the workspace repository. Projects can share that repository or use independent
repositories side by side. Nothing under the standard grouping directories is
ignored by default just because it is a project.

For each independently owned project:

1. Add its exact, root-relative directory to the workspace's `.gitignore`, for
   example `/apps/my-app/`. Use the same pattern for projects under `packages/`,
   `tools/`, `infrastructure/`, `research/`, or a user-owned `templates/`
   grouping; do not ignore the entire grouping directory.
2. Clone the project's repository into that directory, or create it and run
   `git -C apps/my-app init`. WSUP does not initialize Git repositories.
3. Register it in `workspace.json` with `"path": "apps/my-app"` and
   `"repository": "independent"`, along with the other required project fields.
   This metadata does not change Git's tracking or ignore rules.
4. Run `git check-ignore -v apps/my-app/` and review `git status` from the workspace
   root before staging workspace changes. Commit project changes from the
   project's own repository.

The installed generation catalogue under `.wsup/templates/` is WSUP-owned runtime
content, not a project grouping.

The ignore entry keeps the project's files and nested repository out of normal
workspace staging, while workspace-owned sibling projects remain trackable.
Do not force-add independent projects or add them as submodules unless you
intentionally want a different ownership model. Ignoring only nested `.git`
directories is not enough to exclude project contents.

Ignore rules do not untrack files that the workspace repository already tracks.
Before converting an existing project, preserve its work and history, then
deliberately remove its paths from the workspace index (for example,
`git rm -r --cached -- apps/my-app`) and review the staged removals. `--cached`
leaves working files on disk; this step does not transfer their history into
the independent repository.

## Runtime and portability

The CLI uses Python 3.9+ and the standard library. Registered project checks use
executables installed locally and are not translated between operating systems.
Commands may depend on platform-specific executables. Verify the required
checks on each target platform; portability is not a certification claim.

## Configure checks

A check can be a literal argument array:

```json
"checks": {
  "test": ["{python}", "-B", "-m", "unittest", "discover", "-s", "tests"]
}
```

Or an object with `command` plus optional `mode`, `enabled`, `reason`,
`timeout_seconds`, and `finding_exit_codes`. Commands are launched directly,
without a shell, from the registered project's directory. `{python}` selects the
interpreter running the CLI. Other executables must already be installed and
available locally. A command can still read, write, or otherwise act with the
current user's permissions; inspect it before execution.

Checks are `required` by default. A failed required check makes the run fail.
`report` checks can return findings without failing the run when they exit with a
configured finding code (default `1`). Disabled checks need a reason. Projects
and check names must be explicitly selected; dependencies are not run
automatically.

## Local settings and reports

Settings have these defaults:

| Setting | Default |
| --- | --- |
| `timeout_seconds` | `300` (range `1`-`86400`) |
| `output_limit_bytes` | `65536` per stream (range `1`-`10485760`) |
| `report_dir` | `.wsup/reports` (must remain inside `.wsup/reports/`) |

Configuration precedence, from lowest to highest: defaults, `wsup.json`,
`wsup.local.json`, `WSUP_TIMEOUT_SECONDS` / `WSUP_OUTPUT_LIMIT_BYTES` /
`WSUP_REPORT_DIR`, then CLI overrides.
Configuration files use `{ "version": 1, "settings": { ... } }`. Local settings
and `.wsup/reports/` should not be committed. Reports are stored there and
include check outcomes and hashes, not command arguments or captured output.
`--show-output` prints captured output to the terminal.

## Generated files and updates

`editor` creates a VS Code multi-root file with relative paths and refuses to
overwrite an existing destination. VS Code is optional.

`update` compares generated files with their recorded template versions and
previews safe changes. `update --apply` applies changes only when there are no
conflicts. User-edited or missing files are reported for manual review;
unrecognized files are retained. The workspace registry remains user-owned.

### Pre-1.0 layout migration

This is a deliberate layout break without old wrappers or duplicate catalogues.
A trusted newer CLI at `tools/wsup.py` (or installed at `.wsup/wsup.py`)
relocates manifest-owned `tools/wsup.py`, runtime modules, embedded templates,
`.ai/rules/`, and legacy `.ai/skills/`. Redundant root project templates and
unchanged optional grouping scaffolds retire. Other orphaned files and
unrelated project contents are retained.

Preview actions distinguish `relocate` (current template content at the new
path), `retire`, `write`, `move-report` (exact bytes), and `retain-orphan`.
Reports under `.work/quality/` move to `.wsup/reports/` without rewriting
historical paths or hashes. Other project-local `.work/` evidence is untouched.
Explicit report settings under `.work/` block apply; review and change them
before retrying.

Edited or missing relocation/retirement sources, unrecognized files in managed
legacy trees, destination collisions (even identical files), and symlink paths
require manual resolution. Conflicts block all planned writes. The registry
remains user-owned. Only empty ancestors of exact removed files are cleaned up.

Use explicit `--keep PATH`, repeated per file, to preserve existing same-path
customizations such as a workspace map or ignore file. Their contents and
recorded baselines are preserved, not forged to match incoming content.
Repeat the choice on later updates until deliberately reconciled. Retained
optional scaffolds become user-owned when they leave the catalogue. `--keep`
cannot bypass relocation or destination conflicts.

Back up the workspace, including ignored evidence and customizations, then
run a trusted newer source checkout against the old root:

```sh
python3 -B /path/to/new-wsup/tools/wsup.py --root /path/to/existing-workspace update
python3 -B /path/to/new-wsup/tools/wsup.py --root /path/to/existing-workspace update --apply
```

From a newer generated installation use `/path/to/new/.wsup/wsup.py` instead.
After inspecting the preview and applying, use the updated `.wsup/wsup.py` for
`validate`, `guidance`, and repeat preview. Clean migration has no further actions;
explicit keep decisions remain visible.

For customized guidance relocations, preserve edits, perform an owner-reviewed
merge, and recover original managed sources from a trusted backup if necessary.
Never fabricate baseline hashes. If recovery is impossible, retain the original
workspace and plan a transfer into a fresh installation.

Apply is not a filesystem transaction: new files and reports are written before
old sources are removed, and state is saved last. An I/O failure can leave a
partial update. Restore from backup before retrying; no automatic rollback,
downloading, dependency installation, or publication is performed.
