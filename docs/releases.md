# Versioning and releases

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

## Manual release checklist

1. Review the target commit and complete diff. Preserve unrelated work and
   resolve release-blocking issues. Use a clean checkout for release validation.
2. Choose the version using the policy above. Update native `Version` and finalize the
   corresponding [changelog](../CHANGELOG.md) entry, retaining an Unreleased
   section. Describe migrations and material limitations.
3. On the pinned Go toolchain and each claimed runtime target, verify:

   ```sh
   go version
   go run ./cmd/wsup --version
   go run ./cmd/wsup validate
   go run ./cmd/wsup guidance
   go run ./cmd/wsup check --project wsup --show-output
   git diff --check
   ```

   Record the OS/architecture, Go version, and Git prerequisite for development tests.
   macOS ARM64 is currently verified. Cross-compilation is build evidence, not
   runtime certification; non-Unix checks remain unsupported. Report unavailable targets as
   missing coverage; claim client support only to the extent actually tested.
4. Build a standalone binary outside the tracked tree. Generate a disposable
   workspace outside the checkout. Run validation/guidance and child generation
   with that external binary and empty PATH. Workspaces carry state/reports, not
   their own executable or copied catalogue. For layout or
   update changes, exercise an actual supported schema-2 workspace as well as
   synthetic fixtures: preview must be read-only, conflicts must preserve work,
   and successful apply must leave a stable subsequent preview.
5. Check that root and bundled skill copies agree, relative references work,
   and independent repositories remain outside workspace staging. Distinguish
   structural checks from any live client-discovery tests.
6. Review [LICENSE](../LICENSE) and distribution notices. Both starters include
   `WSUP-LICENSE` copies that must match the repository license. Verify generated
   and embedded templates retain the full MIT copyright and permission notice.
   These notices cover WSUP content, not the user's own project license.
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

The current 0.1.0 value is retained during unreleased development, not a published
native release. A compatibility-
changing native release requires the pre-1.0 minor-version policy above; no new
version is selected automatically.

There is no release automation, package publication, or remote self-update
service. Keep development notes and release-preparation drafts under ignored
`workbench/.local/`; CLI check reports are under ignored `.wsup/reports/`. Publish
user-facing changes and material limitations in the changelog or release notes.
See [AI guidance](ai-guidance.md) for compatibility limits and the
[workspace model](workspace-model.md) for upgrade delivery.
