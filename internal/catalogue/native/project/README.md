# Project name

Describe the project's purpose and primary users.

## Requirements

List required runtimes, services, and tools.

## Setup

Give reproducible setup steps and required configuration. Keep secrets out of
source control; provide safe examples where useful.

## Use

Describe the primary command, entry point, or workflow.

## Checks

List the exact test, lint, and validation commands and any known limitations.

## Ownership

Name the maintainer and document dependencies on other projects or services.

## Documentation

Keep maintained documentation with this project, usually in `docs/` or another
established location. Keep one canonical copy and link to it instead of
maintaining duplicates. A journal is optional; a visible `workbench/` can hold
drafts and temporary analysis. Sanitized drafts are eligible for local Git
tracking, but review them before staging; nothing is committed automatically.
Keep restricted records outside documentation and Git.

## Starter attribution

This project starter has no WSUP runtime, binary, catalogue, or workspace registry.
Its `.wsup/template-state.json` schema 2 records managed starter files. A compatible
separately installed native `wsup` can inspect guidance with
`wsup --root /path/to/project guidance`. Preview managed updates with
`wsup --root /path/to/project update`; add `--apply` only after reviewing the
preview and preserving a matched backup of files and state. Conflicts block
apply; repeat `--keep PATH` for eligible same-path customizations without
accepting their current bytes as new baselines. Python cannot update this state.
Do not edit the schema version to force a downgrade. Updates are not transactional;
I/O failures may leave partial content with old state and are reported explicitly.
Project commands and runtime requirements remain this project's responsibility.

The WSUP starter content is covered by [WSUP-LICENSE](WSUP-LICENSE). Retain this
notice when redistributing that content. It does not choose a license for this
project's own code; document that license separately.
