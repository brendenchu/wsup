---
name: prepare-release
description: Prepare a project release record with its version rationale, build provenance, checksums, and bundle review, without tagging or publishing. Use when the user asks to prepare, cut, or check a release.
---

# Prepare a release

1. Identify the project and read its README, AGENTS.md, changelog, and any
   release guide, plus releases/AGENTS.md. Inspect Git status and history since
   the previous release tag. Stop and report if the working tree has changes
   the release would not include.
2. Choose the version. Follow the project's documented versioning policy; if
   it has none, propose a semantic version from the changes since the last
   release and ask the user to confirm it. Check that the changelog has an
   entry for that version that matches the changes.
3. Build with the project's own configured command from a clean checkout of the
   release revision. Record the command, revision, toolchain versions, and
   operating system and architecture.
4. List every artifact and compute its SHA-256. Review each bundle's contents
   for private records, credentials, repository metadata, local paths, and
   unexpected files, and run its smoke check outside the source checkout when
   the project has one.
5. Write `releases/<project>/<version>/RELEASE.md` with the version and its
   rationale, the proposed tag name and revision, the build record, the
   artifact table with checksums, the review and smoke-check results, and a
   link to the changelog entry. Copy artifacts into that folder only when the
   user asks: large binaries belong in a release host, not in Git.

Completion means the record identifies the exact bytes, how they were built,
and what was checked, and every unchecked item is stated. Tagging, pushing,
publishing, and uploading are separate actions the user authorises explicitly.
