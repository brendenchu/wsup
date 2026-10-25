# Changelog

User-visible changes to WSUP are recorded here. One WSUP version covers the CLI,
bundled templates, and portable skills; it does not version users' projects.
See the [release guide](docs/releases.md) for version and tag policy.

## Unreleased

No changes yet.

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
  extraction, and no-toolchain usage instructions. Version 0.2.0 was reissued
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
