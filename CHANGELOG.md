# Changelog

User-visible changes to WSUP are recorded here. One WSUP version covers the CLI,
bundled templates, and portable skills; it does not version users' projects.
See the [release guide](docs/releases.md) for version and tag policy.

## Unreleased

### Changed

- Deliberate pre-1.0 installation layout: generated commands use `.wsup/wsup.py`,
  with runtime and one authoritative template catalogue under `.wsup/`.
  The source checkout retains its visible `tools/` and `templates/` layout.
- Move shared rules to visible `guidance/rules/`, retaining `AI.md`, the
  `AGENTS.md` pointer, and discoverable `.agents/skills/`.
- Store verification reports under ignored `.wsup/reports/`; enforce report
  containment independently from general project-local investigation evidence.
- Omit empty project grouping and journal scaffolds from new workspaces. Create
  collections and dated recap entries on demand.
- Clarify source-checkout versus installed-CLI paths, optional project grouping,
  report locations, and legacy layout migration in maintainer and starter docs.
- Carry reusable documentation ownership, draft tracking, promotion, and privacy
  conventions into generated workspace/project guidance and contributor docs.

### Added

- Preview/apply migration for unchanged, manifest-owned legacy installation paths,
  redundant template copies, and optional scaffolds. Destination collisions,
  customized relocation sources, and unmanaged legacy files block writes.
- Byte-preserving migration of historical `.work/quality/` reports.
- Explicit `update --keep PATH` decisions for same-path customizations and retained
  optional scaffold files, without changing their baseline hashes to hide edits.
- Regression coverage for minimal generation, old-layout migration, retained
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
