# Changelog

User-visible changes to WSUP are recorded here. One WSUP version covers the CLI,
bundled templates, and portable skills; it does not version users' projects.
See the [release guide](docs/releases.md) for version and tag policy.

## Unreleased

### Changed

- Native help/version do not bypass invalid-option validation, unlike Python's
  immediate argparse help/version actions.
- Preserve Python CLI long-option abbreviations and missing-command errors in
  the Go port. Native parser/runtime diagnostic wording may differ while failure
  exits, output roles, and no-write behavior remain compatible.
- Go guidance findings use deterministic lexical error ordering, preserving
  duplicates, instead of Python's filesystem traversal order. Metadata and
  scan-root symlink handling retain the Python inspection contract.
- Refresh current Python CLI documentation: root and destination selection,
  command options and exit codes, check selection and timeout precedence,
  configuration examples, starter boundaries, and source-based update delivery.
- Retire unchanged obsolete app/infrastructure README copies in legacy bundled
  catalogues during updates; preserve user collection docs and block retirement
  of customized catalogue copies.
- Remove unused project-home scaffolds from the product repository, retaining
  them in generated starters. Explicit template-state `omitted_home_files`
  preserves local scaffold omissions during updates without deleting contents.
- Consolidate human working material under workbench: ignored `workbench/.local/`
  holds local notes and handoffs, created on demand. Retain legacy `.work/`
  ignore rules without treating ignored content as disposable or confidential.
- Align the development checkout and both starter Git-ignore baselines, including
  macOS metadata, while retaining repository-local ownership exclusions.
- Define the workspace boundary explicitly: each workspace knows only itself
  and its descendants. Keep documentation, guidance, configuration, and
  onboarding self-contained with no upward workspace relationship or inheritance.
- Deliberate pre-1.0 installation layout: all workspaces use `.wsup/wsup.py`,
  with runtime and one authoritative template catalogue under `.wsup/`.
  The development checkout keeps authoritative implementation and templates
  under `src/`, root `tests/`, and generated installation output under `.wsup/`.
- Move shared rules to visible `guidance/rules/`, retaining `AI.md`, the
  `AGENTS.md` pointer, and discoverable `.agents/skills/`.
- Store verification reports under ignored `.wsup/reports/`; enforce report
  containment independently from general project-local investigation evidence.
- Create a focused development baseline: apps, packages, tools, infrastructure,
  docs, and workbench. Make learning, research, reference, and journal homes
  explicit CLI opt-ins rather than prescribing a knowledge workflow.
- Clarify source-checkout versus installed-CLI paths, default and optional
  collections, report locations, and legacy layout migration in maintainer and
  starter docs.
- Carry reusable documentation ownership, draft tracking, promotion, and privacy
  conventions into generated workspace/project guidance and contributor docs.
- Document workspace collection purposes and boundaries; keep archive,
  artifacts, and private collections opt-in.

### Added

- Native `init DESTINATION` creates an embedded workspace starter; `--template
  project` selects a project. Both use schema-v2 state and no bundled runtime or
  catalogue; projects also omit a registry. Native workspace docs and skills
  describe the separately installed binary and current command limits. Existing
  destinations are refused; I/O failures explicitly retain partial output for
  inspection. Python migration is not yet available; Python cannot update native state.
- Native collection selection and additions: repeatable `init --collection NAME`,
  listing/preview `collections --add NAME`, and explicit conflict-free `--apply`.
  Unmanaged scaffold files are never adopted or overwritten; selections persist
  in state without becoming child-generation defaults.
- Native `update` previews and explicitly applies embedded managed-content
  changes for schema-2 workspaces/projects, preserving registry ownership,
  selected collections, omitted homes, and kept baseline hashes. Conflicts block
  apply; stale inputs and partial I/O failures are explicit errors. Python state
  requires a separate migration and is never converted by content updates.
- Native Unix `check` execution with explicit selection, timeout precedence,
  bounded output, process-group cancellation, and sanitized schema-v2 reports.
  Non-Unix execution remains unsupported; macOS ARM64 is the verified runtime.
- Go migration bootstrap: pinned development toolchain, root Go module, thin
  Cobra executable with help/version, explicit failures for unported commands,
  and byte-verified embedded starter snapshot. Python remains the operational CLI.
- Go inspection commands for registry validation, project listing, effective
  configuration, structural guidance, and exclusive editor-file generation,
  with Python fixture comparison and explicit validation of malformed contracts.
- Optional collection CLI workflows: repeatable `init --collection NAME`
  selections and preview/apply `collections --add NAME` for learning, research,
  reference, journal, archive, artifacts, and private scaffolds. Selections persist in template state
  and the standalone catalogue; existing files are never overwritten by opt-in.
- Preview/apply migration for unchanged, manifest-owned legacy installation paths,
  redundant template copies, and retired layout files. Destination collisions,
  customized relocation sources, and unmanaged legacy files block writes.
- Byte-preserving migration of historical `.work/quality/` reports.
- Explicit `update --keep PATH` decisions for same-path customizations without
  changing their baseline hashes to hide edits.
- Regression coverage for the default layout, old-layout migration, retained
  evidence, conflict safety, report-root symlinks, and source/consumer separation.

## 0.1.0 (2026-10-24)

Initial development release.

### Changed

- Lower the minimum supported Python version from 3.11 to 3.9, including macOS
  developer-tools Python 3.9.6. The CLI still uses only the standard library.
- Clarify WSUP's support for AI and non-AI workflows, open formats, and optional,
  replaceable service integrations in the product and generated-workspace
  documentation.

### Added

- MIT license.
- WSUP-specific MIT notices in both starters, retained through standalone
  generation without setting the license of user projects.
- Generic project ignore rules for local environment files, work records, and
  Python caches, while keeping `.env.example` trackable.
- Contribution instructions and a manual versioning and release checklist.
- Local-first project registry, explicit checks, settings, and local reports.
- Optional VS Code workspace-file generation.
- Standalone workspace/project starters and conflict-aware template updates.
- Client-neutral guidance and general workflow skills.
- Domain skills for project onboarding and Git ownership, registry and checks,
  template lifecycle, and AI guidance maintenance.
- Guidance for independent project repositories and explicit per-project ignore
  entries.
- Regression coverage for mixed-Git onboarding and legacy skill migration.
- Portable skills under `.agents/skills/`, with `.ai/rules/` for constraints and
  task routing.
- Previewable relocation of unchanged, manifest-owned legacy skill files.
  Conflicts, including edited or missing files and destination collisions, block
  apply. Updates do not download releases or provide transactional rollback.

### Compatibility and limitations

- Requires Python 3.9 or newer and locally installed tools for registered checks.
- Existing workspaces using `.ai/skills/` must preview updates with a trusted
  newer CLI. Only unchanged, manifest-owned legacy skills can relocate
  automatically; customized layouts require manual resolution. The pre-1.0
  layout migration also moves `.ai/rules/` and the installed runtime. See the
  [upgrade and recovery instructions](docs/workspace-model.md).
- AI-client discovery and operating-system compatibility require validation in
  the actual environment; structural guidance checks do not certify either.
- Retain the bundled `WSUP-LICENSE` when redistributing WSUP code or substantial
  template content; this does not set the license of user projects.
