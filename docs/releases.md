# Versioning and releases

## Current release

[WSUP v0.2.0](https://github.com/brendenchu/wsup/releases/tag/v0.2.0), dated
2026-10-25, is the published native release. It includes a compiled macOS ARM64
archive and `SHA256SUMS`; see [download and extraction](native-install.md).
Users do not compile the downloaded binary. Go is needed for source development
and release packaging, not normal binary use.

Version 0.2.0 was reissued with corrected distribution documentation and a rebuilt
archive by explicit maintainer authorization. Its tag and asset checksums changed;
download the archive and manifest together from the release page rather than
combining a retained earlier asset with a new manifest. This is an exception to
the immutable-tag policy below, not the normal release procedure.

## One product version

WSUP uses one version for its CLI, bundled workspace/project templates, and
portable skills. The native authoritative value is `Version` in
[internal/cli/cli.go](../internal/cli/cli.go). It supplies CLI
`--version` output and template-version reporting.

Template-state schema `version` is a separate format identifier; a product
release does not require changing that schema number. Per-file hashes determine
template update actions. A version label or Git tag alone does not fetch new
content or deliver an update.

User projects retain their own versions, Git histories, and release cycles.

## Version policy

Use `MAJOR.MINOR.PATCH` versions:

- During `0.x`, increment the minor version for new capabilities or compatibility
  changes, especially CLI contracts, generated layouts, and migration behavior.
- Increment the patch version for backward-compatible fixes.
- After a deliberate 1.0 stability decision, use major versions for incompatible
  public changes, minor versions for compatible features, and patches for fixes.

Template and skill behavior count as product behavior. Routine documentation
edits can accumulate under Unreleased until the next product release.

Commits are work increments; they do not each require a new version. Group
changes into a release, update the version and changelog together, and tag the
exact reviewed commit. Do not infer versions automatically from commit subjects.

## Tags

Use annotated tags named `vMAJOR.MINOR.PATCH`, matching the committed product
version. Once shared, treat a release tag as immutable; correct mistakes with a
new release rather than moving a published tag.

Changelog entries alone are not proof of release validation. If a historical
baseline is tagged retrospectively, identify it as such and state what was
actually verified.

## Local candidate archive

The [native binary guide](native-install.md) covers extraction, explicit-path
use, owner-selected installation, replacement, uninstall, and recovery. It is
also the archive's self-contained `README.md`. This procedure builds a local
macOS ARM64 candidate, not a release, and leaves product `Version` unchanged.
Use Go 1.27.3 and cached dependencies. Packaging also requires Bash, Git, POSIX
shell tools, `tar`, `gzip`, and `shasum`; these are not application runtime
requirements. The command refuses hosts other than macOS ARM64 and a Go version
different from `go.mod`. It uses offline dependencies and never installs tools.

Run from the source root. The fresh temporary output directory avoids replacing
owner files and stays outside the tracked tree:

```sh
mise exec go@1.27.3 -- bash scripts/package.sh
```

If the pinned Go toolchain is already on PATH, `bash scripts/package.sh` is
sufficient. The script runs the full race suite, vet, registry validation and
guidance checks, compares two release-profile binary builds, packages full
notices, verifies checksums and extraction, and exercises the extracted binary
with empty PATH. Smoke coverage includes generation, guidance, collection addition,
updates, relocation, and child workspace/project generation.

The printed output directory contains the package, archive, and `SHA256SUMS`.
Successful runs remove their disposable verification fixtures; failures retain
staging for inspection and exit nonzero. The script accepts no arguments,
does not bump versions, and never tags, pushes, publishes, installs, signs, or
changes PATH. It can package uncommitted work, recording Git revision/status in
`BUILD.txt`; such output is a local candidate, not exact-commit release evidence.
It rejects observed revision/status changes during the run, but does not isolate
concurrent writes or detect every edit to already-dirty files. Keep source
unchanged while packaging and use an isolated clean checkout for release work.

`-trimpath` removes source paths and `-buildvcs=false` avoids implicit VCS metadata;
the separate build record supplies provenance. Cgo is disabled. Native acceptance
checks identical binary bytes from repeated builds of the same isolated source
with this profile and toolchain. This is not a guarantee across Go versions or
different source inputs. Tar metadata/compression can vary: archive hashes
identify the actual artifact, not reproducible archive bytes.

The notice set conservatively includes the pinned Go toolchain's vendored
components and all declared external runtime modules, even modules not linked
on macOS. Recheck it against `go list -deps ./cmd/wsup`, other selected targets,
and upstream LICENSE/NOTICE requirements whenever toolchain/dependencies change.
Do not substitute a dependency summary for required full notices.

Extract into another fresh directory, verify its inventory and executable mode,
and run the exact extracted binary outside the checkout with empty PATH.
Exercise help/version, generation, guidance, relocation, collection addition, update
preview/apply, and explicit checks using a separately supplied test executable.
Confirm reports describe the native runtime and preserve owner customizations.
Never use a real workspace as a packaging fixture.

Keep outputs only as long as needed; remove only the specific disposable files
and directories created for this exercise. A final release must rebuild from
the selected clean commit with its newly chosen version, omit the `-local`
candidate suffix, and repeat all target verification. Local candidates do not
authorize a version bump, tag, installation, signing, or publication.

## Manual release checklist

Enable repository release immutability before the next publication. It applies
to future releases, not retroactively to v0.2.0. Prepare the release as a draft
and finish asset/checksum verification before publishing: publication locks the
release assets and associated tag. Correct a published immutable release with
a new version; do not rely on disabling the setting to unlock it.

1. Review the target commit and complete diff. Preserve unrelated work and
   resolve release-blocking issues. Use a clean checkout for release validation.
2. Choose the version using the policy above. Update native `Version` and finalize the
   corresponding [changelog](../CHANGELOG.md) entry, retaining an Unreleased
   section. Describe migrations and material limitations.
3. On the pinned Go toolchain and each claimed runtime target, verify:

   ```sh
   go version
   go run ./cmd/wsup --version
   go test -race ./...
   go vet ./...
   git diff --check
   ```

   Record the OS/architecture, Go version, and Git prerequisite for development tests.
   macOS ARM64 is currently verified. Cross-compilation is build evidence, not
   runtime certification; non-Unix checks remain unsupported. Report unavailable targets as
   missing coverage; claim client support only to the extent actually tested.
4. Follow the local archive procedure, including required notices and checksums.
   Build a standalone binary outside the tracked tree. Generate a disposable
   workspace outside the checkout. Run validation/guidance and child generation
   with that external binary and empty PATH. Workspaces carry state/reports, not
   their own executable or copied catalogue. For layout or
   update changes, exercise an actual supported schema-2 workspace as well as
   synthetic fixtures: preview must be read-only, conflicts must preserve work,
   and successful apply must leave a stable subsequent preview.
5. Check the canonical skills in `templates/skills/`, their owning module
   manifests, and the skills deployed into disposable starters. There is no
   root skill library in this source repository. Confirm relative references
   work and independent repositories remain outside workspace staging. Distinguish
   structural checks from any live client-discovery tests.
6. Review [LICENSE](../LICENSE) and distribution notices. Generated starters
   carry no license file; confirm none has been reintroduced under `templates/`.
7. With explicit authorization, commit the release changes. From the clean,
   validated release commit, create the annotated tag for the newly selected
   version. Replace `MAJOR.MINOR.PATCH` below with that exact version; never move
   or reuse the historical `v0.1.0` tag for the native release:

   ```sh
   git tag -a vMAJOR.MINOR.PATCH -m "WSUP MAJOR.MINOR.PATCH"
   git show --no-patch vMAJOR.MINOR.PATCH
   ```

8. Publish or push only with explicit authorization. Tag creation is local and
   does not publish a release. If pushing is approved, specify the intended
   remote and exact tag rather than pushing every local tag.

9. Build the final archive from the exact clean tagged commit, without the
   `-local` suffix. Include the binary, full notices, native binary guide, and a
   build record identifying the tag, commit, toolchain and build metadata.
   Repeat extracted-binary verification and generate `SHA256SUMS` for that exact
   archive. With publication authorization, create a draft GitHub release for
   that tag, upload both assets and user-facing release notes, and verify the
   complete draft inventory against the validated local files. Publish only
   after that verification; do not publish an empty release and attach assets
   afterward. Publication and the next version remain explicit maintainer actions.
10. Download the published archive and manifest into a fresh directory. Verify
    their hashes and byte identity against the validated local assets. Confirm
    the release tag resolves to the tested commit and the release is not a
    draft. Do not declare download-back verification complete if it was blocked.

Local candidate verification and packaging are automated; there is no automatic
release publication, package-manager publication, or remote self-update service.
GitHub binary release assets are published manually. Keep development notes
and release-preparation drafts outside this repository. Publish
user-facing changes and material limitations in the changelog or release notes.
See [AI guidance](ai-guidance.md) for compatibility limits and the
[workspace model](../templates/modules/workspace-base/files/docs/workspace-model.md) for upgrade delivery.
