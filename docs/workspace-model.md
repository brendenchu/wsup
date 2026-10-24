# Workspace model

The directory layout groups work by responsibility. `workspace.json` records
project ownership and local checks. Git topology is independent: one workspace
may contain a single repository, independent repositories, or a mixture.
Directories are optional, and no shared build or release is implied.

## Register projects

`workspace.json` is UTF-8 JSON with `version: 1`, a nonempty `name`, and a
`projects` array. Each project has:

| Field | Meaning |
| --- | --- |
| `id` | Unique identifier using letters, digits, `.`, `_` or `-` |
| `path` | Existing workspace-relative directory containing `README.md` |
| `kind` | `app`, `package`, `tool`, `infrastructure`, `research` or `template` |
| `repository` | `workspace` or `independent`, recording intended ownership |
| `purpose`, `owner` | Nonempty project description and responsible owner |
| `dependencies` | Optional array of other project IDs |
| `checks` | Optional named check definitions |

Paths use relative forward slashes and must remain inside the workspace, including
through symlinks. Project paths must be unique. Dependencies must resolve and be
acyclic. The registry describes intent; it does not create or verify Git
repositories, clone projects, install dependencies, or release packages.

## Git ownership

Keep workspace configuration, guidance, and the grouping-directory README files
in the workspace repository. Projects can share that repository or use independent
repositories side by side. Nothing under the standard grouping directories is
ignored by default just because it is a project.

For each independently owned project:

1. Add its exact, root-relative directory to the workspace's `.gitignore`, for
   example `/apps/my-app/`. Use the same pattern for projects under `packages/`,
   `tools/`, `infrastructure/`, `research/`, or `templates/`; do not ignore the
   entire grouping directory.
2. Clone the project's repository into that directory, or create it and run
   `git -C apps/my-app init`. WSUP does not initialize Git repositories.
3. Register it in `workspace.json` with `"path": "apps/my-app"` and
   `"repository": "independent"`, along with the other required project fields.
   This metadata does not change Git's tracking or ignore rules.
4. Run `git check-ignore -v apps/my-app/` and review `git status` from the workspace
   root before staging workspace changes. Commit project changes from the
   project's own repository.

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

The CLI uses Python 3.11+ and the standard library. Registered project checks use
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
| `report_dir` | `.work/quality` (must remain inside `.work/`) |

Configuration precedence, from lowest to highest: defaults, `wsup.json`,
`wsup.local.json`, `WSUP_TIMEOUT_SECONDS` / `WSUP_OUTPUT_LIMIT_BYTES` /
`WSUP_REPORT_DIR`, then CLI overrides.
Configuration files use `{ "version": 1, "settings": { ... } }`. Local settings
and `.work/` should not be committed. Reports are stored within `.work/` and
include check outcomes and hashes, not command arguments or captured output.
`--show-output` prints captured output to the terminal.

## Generated files and updates

`editor` creates a VS Code multi-root file with relative paths and refuses to
overwrite an existing destination. VS Code is optional.

`update` compares generated files with their recorded template versions and
previews safe changes. `update --apply` applies changes only when there are no
conflicts. User-edited or missing files are reported for manual review;
unrecognized files are retained. The workspace registry remains user-owned.

### Legacy skill layout migration

New workspaces use `.agents/skills/`; `.ai/rules/` remains the rule location.
Workspace updates can relocate unchanged, manifest-owned legacy `.ai/skills/`
files and their embedded workspace-template copies. Preview reports each
`relocate` action with its destination `path` and legacy `from` path. Applying
writes the current template content, then removes only those exact legacy files.
Empty old directories can remain. Other removed template files are still retained.

Edited or missing legacy files, unrecognized files in a legacy skill tree,
destination collisions (even identical files), and symlink paths require manual
resolution. Conflicts block the entire apply before any planned writes. Preview
is read-only; the registry and unrelated files remain user-owned.

An old generated CLI does not download a newer release or template bundle. To
upgrade, obtain a trusted newer WSUP checkout or generated workspace separately,
back up the existing workspace, and run that newer CLI against the old root:

```sh
python3 -B /path/to/new-wsup/tools/wsup.py --root /path/to/existing-workspace update
python3 -B /path/to/new-wsup/tools/wsup.py --root /path/to/existing-workspace update --apply
```

Inspect the preview before applying. After a successful apply, use the updated
workspace's own CLI for `validate` and `guidance`, and review Git status.
Repeated preview should report no further changes.

If legacy skills were customized, preserve those edits outside the managed
trees, restore the recorded baseline from a trusted backup or original template,
resolve destination collisions, then preview again. Reapply desired edits to
the canonical files after migration and validate guidance. Do not fabricate
manifest hashes to bypass conflicts. If the baseline cannot be recovered, keep
the old workspace intact and plan a manual transfer into a fresh workspace.

Apply is not a filesystem transaction: an I/O failure can leave some new files
written. Legacy files are removed only after the planned writes succeed, and
the manifest is saved last. On failure, inspect the error and restore from the
pre-upgrade backup before retrying; there is no automatic rollback command.
