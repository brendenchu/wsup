# Migrating from wsup 0.2.0

wsup 0.3.0 changes the workspace layout: `workspace.json` and `.wsup/` become
`wsup.json` and `wsup.lock`, `AI.md` and `guidance/rules/` fold into
`AGENTS.md`, skills are renamed, and the default homes become collections.
Commands that read `wsup.json` or `wsup.lock` refuse a 0.2.0 workspace and
say it needs converting. `wsup migrate` converts a workspace or a project starter in place,
with a preview first and a backup of everything it replaces.

## Before you start

1. Get a wsup 0.3.0 binary ([native binary use](native-install.md)) and check
   `wsup --version`.
2. Commit or stash your work so `git status` is clean. The migration backs up
   the files it replaces, but Git is the easiest way to review the result.
3. Run `wsup --root WORKSPACE validate` with the old 0.2.0 binary if you still
   have it, and fix anything it reports first. A registry that 0.2.0 already
   rejected (for example a project path that is a symlink out of the
   workspace) is rejected by the conversion too.

Migrate each independent project starter separately, from its own root.

## 1. Preview

```sh
wsup --root WORKSPACE migrate
```

The preview writes nothing. It lists:

- **Actions:** files replaced with their 0.3.0 version (you never edited them),
  files removed because 0.3.0 no longer installs them (`AI.md`,
  `guidance/rules/`, old-name skills, `WSUP-LICENSE`, `.gitkeep` homes), and
  files written for the first time.
- **Kept files:** files you edited. They are never touched. One that 0.3.0
  still installs, such as `README.md`, keeps its 0.2.0 baseline in `wsup.lock`,
  so `update` reports it afterwards.
- **Notes:** what you need to do by hand, such as folding an edited `AI.md` or
  rule file into `AGENTS.md`, or moving a renamed folder's content.
- **Conflicts:** files you created at a path 0.3.0 installs. Any conflict
  blocks every write.

The registry conversion is part of the preview: `workspace.json` and the
settings-only `wsup.json` become one `wsup.json` (version 3). Checks become
tasks with `"check": true`, project kind `infrastructure` becomes `infra`, and
a `report_dir` under `.wsup/reports` moves under
`workbench/.local/wsup/reports` (in `wsup.local.json` too).

## 2. Resolve conflicts

A conflict is a file you wrote at a path wsup now installs, for example an
owner-written `CLAUDE.md` or `apps/README.md`. For each one, either:

- keep yours: repeat `--keep PATH` on the migrate command. The file is left as
  it is and recorded against wsup's version, so a later `update` reports it
  until `update --keep PATH` accepts it; or
- take wsup's: move your file aside (its content is in Git) and preview again.

```sh
wsup --root WORKSPACE migrate --keep CLAUDE.md --keep apps/README.md
```

Adapters default to all, as at `init`. Pass `--agent NAME`, `--agent none`,
`--editor NAME`, or `--editor none` to choose; an owner-written `CLAUDE.md`
or `GEMINI.md` is a conflict only if its adapter is selected.

## 3. Apply

```sh
wsup --root WORKSPACE migrate --apply
```

(Add the same `--keep`, `--agent`, and `--editor` options as the preview.)
Every file to be replaced or removed is first copied to
`workbench/.local/wsup/migrate/<UTC timestamp>/`. Then the content is written,
`.wsup/reports/` moves to `workbench/.local/wsup/reports/`, `wsup.lock` is
written, and the 0.2.0 state in `.wsup/` is removed last. If anything fails
partway, restore from the backup folder (or Git) before retrying.

## 4. Finish by hand

- **Fold edited guidance.** For each edited `AI.md` or `guidance/rules/` file
  the notes list, move the rules you still want into `AGENTS.md` (outside the
  `<!-- wsup:begin NAME -->` markers) or a collection's `AGENTS.md`, then
  delete the old file. Procedural rules may fit better as a skill in
  `.agents/skills/`.
- **Move renamed folders.** `infrastructure/` and `artifacts/` are never moved
  when they hold your content; the selection gains `infra` and `releases` and
  the notes say what to move. Move the content, update the affected `path`
  entries in `wsup.json`, and delete the empty old folder. `private/` is not a
  0.3.0 collection and is left alone; keep restricted records outside the
  workspace.
- **Review old-name skills you edited** (`repository-change`,
  `commit-message`, and others). The notes name each replacement; carry your
  changes over, then delete the old folder.

## 5. Verify and commit

```sh
wsup --root WORKSPACE validate
wsup --root WORKSPACE guidance
wsup --root WORKSPACE update
```

`validate` and `guidance` should report no errors. `update` should list only
the edited files you kept; accept each with `update --keep PATH --apply` or
replace it. Run a check (`wsup check --project ID`) to confirm tasks still
work and reports land in `workbench/.local/wsup/reports/`. Then review
`git status` and commit `wsup.json`, `wsup.lock`, and the converted files.
The backup under `workbench/.local/` is ignored; delete it when you no longer
need it.

## What changed

| 0.2.0 | 0.3.0 |
| --- | --- |
| `workspace.json` + settings-only `wsup.json` | `wsup.json` version 3 (registry, settings, selected modules) |
| `.wsup/template-state.json` | `wsup.lock` (committed) |
| `.wsup/reports/` | `workbench/.local/wsup/reports/` |
| `AI.md`, `guidance/rules/` | `AGENTS.md`, one per collection folder, and skills |
| Project `checks` | Project `tasks`; `"check": true` runs under `wsup check` |
| `wsup check --check NAME` | `wsup check --task NAME` |
| `wsup collections --add` | `wsup add` |
| `wsup editor` | `wsup vscode` |
| `infrastructure/`, kind `infrastructure` | `infra/` collection, kind `infra` |
| `artifacts/` | `releases/` collection |
| `private/` | Removed |
| `WSUP-LICENSE` | Removed |
| Skills `workspace-check`, `registry-checks`, `template-lifecycle`, `project-onboarding`, `ai-guidance-maintenance`, `repository-change`, `commit-message` | `check-workspace`, `run-checks`, `update-workspace`, `onboard-project`, `maintain-agents-md`, `make-change`, `write-commit` |

The full list is in the [changelog](../CHANGELOG.md).
