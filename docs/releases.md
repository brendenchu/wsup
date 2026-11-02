# Versioning and releases

## Current release

[WSUP v0.3.0](https://github.com/wavykit/wsup/releases/tag/v0.3.0), dated
2026-10-31, is the published native release. It includes a compiled macOS ARM64
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

[GoReleaser 2.19.0](https://goreleaser.com/) and Go 1.27.3 are pinned in
`mise.toml`; use those installed tools and cached Go modules. GoReleaser owns
builds, tar.gz archives, checksums and Homebrew cask generation through
[.goreleaser.yaml](../.goreleaser.yaml). The
[release helper](../scripts/release/main.go) prepares the complete notice set,
records provenance, checks repeated binary bytes and verifies the extracted
archive. No publishing workflow is installed by this configuration.

From the source root on macOS ARM64:

```sh
mise exec -- goreleaser check
mise exec -- goreleaser release --snapshot --clean
mise exec -- go run ./scripts/release verify dist
```

Without Mise, run the same commands using the pinned executables on PATH.
GoReleaser uses ignored `dist/`; `--clean` replaces that selected output
directory. The before hook prepares notices in ignored `.release-staging/`,
which the archive verifier also uses; preserve it until verification finishes.
Run from an isolated checkout when release output must stay outside the working
checkout. Keep owner files out of the distribution directory.

The before hooks check the runtime/toolchain and run the full race suite and vet.
Builds use cgo disabled, `-trimpath -buildvcs=false`, no time-dependent linker
values, and the Git commit timestamp for file modification times. The post-build
hook compares a second binary build and exercises the standalone binary with
empty PATH. Snapshots may include local changes; BUILD.txt records the source
revision, dirty status, source-content fingerprint and both artifact/product
versions. The product version is not changed by packaging. Snapshot archive
versions follow the latest tag plus `-local` and may differ from an untagged
working product version; they are not published releases.

The verifier is mandatory after GoReleaser finishes: it validates SHA256SUMS,
rejects any archive inventory other than the expected 17 regular files, verifies
prepared notices/README/provenance byte identity, extracts exclusively into a
fresh temporary directory, and repeats the extracted-binary smoke checks.
Coverage includes init, validate, guidance, collection addition, update, relocation
and child workspace/project generation with empty PATH. It also rejects source
changes since preparation. These are local artifact checks, not installed-client
skill discovery or certification for other platforms.

GoReleaser OSS has no post-archive verification hook in this configuration, so
archive verification is an explicit second command. Successful generation alone
is not final acceptance. Failures retain the selected artifact directory for
inspection; temporary smoke/build fixtures are removed by the helper. Source
fingerprints detect changes to tracked and unignored source bytes during a run;
this is not filesystem isolation or power-loss durability. Use a clean, isolated
checkout for final release work.

Archives keep the existing layout: `wsup_VERSION_darwin_arm64/` with the
executable, LICENSE, the [native guide](native-install.md) as README.md,
BUILD.txt and thirteen full dependency notice files. No root source README,
repository metadata or extended-attribute sidecar files belong in the archive.
Notice inventory conservatively includes the pinned toolchain's vendored
components and declared runtime modules; re-review it when dependencies change.
Repeated binary identity is scoped to the same source and toolchain; archive
byte identity across different runs is not claimed.

## Homebrew definition and publication boundary

GoReleaser generates `dist/homebrew/Casks/wsup.rb` for the same archive.
It uses the supported `homebrew_casks` integration; upstream's old prebuilt-binary
`brews` formula integration is deprecated/removed. `skip_upload: true` keeps
that definition local. The selected future destination defaults to
`wavykit/homebrew-tap`; `WSUP_TAP_OWNER` can override the owner for a separately
authorized destination. This selection does not establish that the tap exists
or is published. A tap owner is a GitHub user or organization,
independent of the Go module domain. Creating, publishing or changing a remote
tap remains separately authorized WSUP installation work.

The cask refers to the release asset URL, so a local snapshot test substitutes
only the download URL with the exact local snapshot archive, retaining the
generated version, checksum and binary stanza. Use an isolated Homebrew prefix
and local tap for the test; preserve the normal installed wsup and tap state.
No automatic quarantine-removal hook is generated. Packages remain unsigned and
not notarized; establish the intended trusted install experience before public
Homebrew announcement. The local prebuilt-cask test installed and removed the
package, but Gatekeeper blocked launching its quarantined ad-hoc-signed binary.
Installed execution is therefore not accepted yet. Developer ID signing and
notarization, or a separately verified source-built formula, need an explicit
installation decision before public Homebrew launch. Do not infer a usable
installed path from successful archive or extraction checks.

Full tap distribution, upgrade/uninstall checks and
canonical source installation are the installation milestone, not implied by
local cask generation.

`release.disable: true` and cask `skip_upload: true` prevent remote publication,
even on a non-snapshot command. A separately authorized release builds from its
clean annotated tag with `goreleaser release --clean`, runs the mandatory verifier,
reviews every asset, then uploads a draft and publishes using the manual gates
below. GoReleaser's local configuration does not grant write permission to any
remote repository or release.

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
4. Follow the GoReleaser archive procedure and mandatory verifier, including
   required notices and checksums.
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

9. Build with `goreleaser release --clean` from the exact clean tagged commit
   (the helper rejects dirty or mismatched-version releases), then run
   `go run ./scripts/release verify dist`. The archive has no `-local` suffix. Include the binary, full notices, native binary guide, and a
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

GoReleaser and its verifier automate local candidate checks and packaging; there is no automatic
release publication, package-manager publication, or remote self-update service.
GitHub binary release assets are published manually. Keep development notes
and release-preparation drafts outside this repository. Publish
user-facing changes and material limitations in the changelog or release notes.
See [AI guidance](ai-guidance.md) for compatibility limits and the
[workspace model](../templates/modules/workspace-base/files/docs/workspace-model.md) for upgrade delivery.
