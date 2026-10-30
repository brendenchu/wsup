# Changelog

User-visible changes to WSUP are recorded here. One WSUP version covers the CLI,
bundled templates, and portable skills; it does not version users' projects.
See the [release guide](docs/releases.md) for version and tag policy.

## Unreleased

### Added

- New opt-in collections: `sandbox`, `templates`, `forks`, `decisions`,
  `issues`, `ai`, `design`, `content`, and `data`. `decisions` and `issues`
  include a template.
- Workspaces and projects include `.editorconfig` (UTF-8, LF, final newline,
  trimmed trailing whitespace) and `.gitattributes` (`* text=auto eol=lf`).
- Collections can declare ignore entries, written to a marked block in the
  workspace `.gitignore`. `data` ignores `data/**/*.local.*`.
- Agent adapters: `claude` (`CLAUDE.md` importing `AGENTS.md`, and a
  `.claude/skills` link to `.agents/skills`) and `gemini` (`GEMINI.md`).
  `init` installs both by default; `--agent NAME` selects and `--agent none`
  skips them. `.claude/skills` is the only symlink wsup creates: `update`
  re-creates it when missing, and `validate` and `guidance` report a missing
  or broken link.
- Editor adapter `vscode`: `init` generates `<name>.code-workspace` from
  `wsup.json` (root plus one folder per project, one VS Code task per project
  task, and the `vscode` block's settings, extensions, tasks, and launch
  configurations) and `update` refreshes
  it, reporting hand edits as conflicts. Renaming the workspace replaces the
  file: `update` removes the old one while it is unedited. `--editor NAME` selects and
  `--editor none` skips it; editor adapters are workspace-only. `wsup editor`
  writes the same document.
- In a terminal, `wsup init` asks for the preset, agents, and editors that
  flags leave open (a project: agents only), re-asking after an invalid
  answer. `--yes`, or running without a terminal, uses the defaults.
- `wsup init --preset NAME` selects a preset of collections; explicit
  `--collection` choices add to it. `--preset none` creates a base-only
  workspace. Unknown presets fail before anything is written.
- `wsup check --all` runs the check tasks of every project.
- `wsup add MODULE...` previews adding collections or agent and editor
  adapters (with any modules they require, listed under `installs`); `--apply`
  installs them, creating the `.claude/skills` link where needed. It works in
  workspaces and, for agents, in projects.
- `wsup modules` lists built-in modules; `wsup modules ID` shows a module's
  manifest and files.
- One-command local macOS ARM64 candidate verification and packaging with the
  pinned Go toolchain, full notices, checksums, repeated binary builds, and
  extracted-binary smoke checks. Tagging and publication remain explicit.

### Changed

- `wsup.json` (version 3) is the workspace file, replacing `workspace.json` and
  the settings-only `wsup.json`. It records the workspace name, the selected
  collections, agents, and editors, shared `settings`, and projects. Projects
  declare `tasks` instead of `checks`; a task with `"check": true` runs under
  `wsup check`, and `--task` replaces `--check`. Tasks keep every check option
  and add `cwd`. Projects may record a `git` URL.
- Project kinds are `app`, `package`, `tool`, `infra`, `sandbox`, `template`,
  `fork`, and `research` (`infrastructure` is now `infra`).
- `wsup.lock` (committed) replaces `.wsup/template-state.json`. It records each
  installed module and version, and baseline hashes for managed files, their
  generated sections, and links. Workspaces created by wsup 0.2.0 report that
  they need converting.
- Check reports default to `workbench/.local/wsup/reports/`, and `report_dir`
  must stay inside it.
- Adding a module also records it in `wsup.json`, rewritten in canonical form.
- `apps/`, `packages/`, `tools/`, and `infra/` are collections instead of
  part of the workspace base, and the default `development` preset selects
  all four. Existing workspaces keep managing `apps/`, `packages/`, and
  `tools/` on update.
- `infrastructure/` is now `infra/`, and `artifacts` is now `releases`.
  `apps/` and `infra/` get a README instead of an empty placeholder. Workspaces
  that selected `artifacts` or `private` report that they need a migration.
- Skills are renamed verb-first: `check-workspace`, `run-checks`,
  `update-workspace`, `onboard-project`, `maintain-agents-md`, `make-change`,
  and `write-commit` (formerly `workspace-check`, `registry-checks`,
  `template-lifecycle`, `project-onboarding`, `ai-guidance-maintenance`,
  `repository-change`, and `commit-message`).
- Workspaces get seven base skills; the five code-change skills come with the
  project collections, so a workspace without one (such as `--preset none`)
  has no code-change skills. Adding a collection installs its skills.
- The project starter includes six skills: the code-change skills and
  `plan-work`.
- `AGENTS.md` is the instructions file for workspaces and projects. It absorbs
  `AI.md` and the `guidance/rules/` text, and its Layout and Skills sections are
  generated between `<!-- wsup:begin NAME -->` / `<!-- wsup:end NAME -->`
  markers. Procedural rule text moved into the skills that use it.
- Every collection folder has its own `AGENTS.md` with that folder's rules.
- When adding a collection or updating, an edited `AGENTS.md` or `.gitignore`
  keeps the owner's text and only its generated sections are replaced (a
  `merge` action), as long as those sections are unchanged.
- The project starter gains `docs/` and `workbench/`.
- `wsup guidance` requires `AGENTS.md` instead of `AI.md` and the rule index,
  inspects top-level folder `AGENTS.md` files, and reports unpaired markers.

### Removed

- `wsup collections`; use `wsup modules --kind collection` and `wsup add`.

- The `.wsup/` folder, and the `.work/` entries in generated `.gitignore` files.

- `AI.md`, `guidance/rules/`, and `WSUP-LICENSE` from generated workspaces and
  projects. Generated output carries no license file.
- The `private` collection. Its name suggested protection it never provided;
  keep restricted records outside the workspace.

### Developer Experience

- The repository is no longer run as a wsup workspace: root `AI.md`,
  `guidance/`, `.agents/skills/`, `workspace.json`, and `workbench/` are
  removed, and `templates/` is the single source for generated content.
  Contributor and agent instructions are in a short `AGENTS.md`.
- CI and local packaging validate a freshly generated workspace instead of
  the repository root.
- Starter content is organized as modules: each has a validated
  `module.json` manifest and its files under `templates/modules/`, and skills
  live once in a shared `templates/skills/` library. Generated output is
  unchanged.

## 0.2.0 (2026-10-25)

### Changed

- Use a standalone Go executable with directly embedded workspace, project, and
  collection templates. Generated workspaces carry managed state and reports,
  not an executable, runtime, or copied catalogue.
- Require schema-2 managed state. Unsupported schemas fail without writes;
  in-place format conversion and obsolete installation/layout cleanup are not
  supported. Create a new destination and explicitly transfer owner content.
- Pass check arguments literally without token substitution. Required tools
  must be installed separately and available by explicit path or on PATH.
- Keep development sources in `cmd/`, `internal/`, and `templates/`, with
  colocated Go tests. Development checks run the Go race suite and vet.
- Use deterministic lexical guidance findings, preserving duplicates. Invalid
  options fail even alongside help/version; unambiguous long-option prefixes
  remain supported.
- Keep rules under `guidance/rules/`, portable skills under `.agents/skills/`,
  reports under ignored `.wsup/reports/`, and local work under
  `workbench/.local/`. Preserve workspace independence and owner customizations.
- Use apps, packages, tools, infrastructure, docs, and workbench as the default
  development homes. Knowledge/lifecycle collections remain explicit opt-ins.

### Added

- Published macOS ARM64 binary archive and SHA-256 manifest, with download,
  extraction, optional PATH setup, and a first-workspace tutorial without a
  Go toolchain. Version 0.2.0 was reissued
  with corrected distribution documentation and a rebuilt archive; download
  its archive and checksum manifest together from the release page.
- Manual local candidate packaging with SHA-256 verification, full runtime
  dependency notices, build provenance, and explicit binary replacement/recovery
  instructions. Release-profile acceptance checks repeatable binary bytes with
  cgo disabled and source paths removed; local candidate packaging does not
  install or publish anything.
- Native initialization, registry/configuration inspection, guidance checks,
  exclusive editor generation, collection selection/additions, and managed
  content update preview/apply with explicit conflict handling.
- State-last updates, stale-input checks, exclusive new files, retained orphan
  bytes, omitted-home selections, and repeatable `--keep PATH` decisions without
  adopting customized baseline hashes.
- Unix process-group supervision, cancellation, bounded transient output, and
  sanitized schema-2 reports with native runtime metadata. macOS ARM64 is
  verified; non-Unix check execution remains unsupported.
- Offline isolated source-build and suite acceptance using only Go/Git, plus
  standalone generation and updates with empty PATH. Canonical guidance,
  embedded assets, hidden files, and license notices have exact parity tests.

### Removed

- The previous implementation, duplicate installation/catalogues, reference
  runners and fixtures, and obsolete format/layout conversion commands.

## 0.1.0 (2026-10-24)

Initial development release: local-first project registry, explicit checks,
settings and reports, optional VS Code workspace generation, standalone starters,
conflict-aware updates, MIT licensing, and client-neutral rules and skills.

Project ownership remains explicit; optional integrations and client discovery
require validation in the actual environment. Retain `WSUP-LICENSE` when
redistributing substantial starter content; it does not set a user project's
license.
