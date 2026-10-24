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
