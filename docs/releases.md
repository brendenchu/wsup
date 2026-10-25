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

## Local candidate archive

The [native binary guide](native-install.md) covers extraction, explicit-path
use, owner-selected installation, replacement, uninstall, and recovery. It is
also the archive's self-contained `README.md`. This procedure builds a local
macOS ARM64 candidate, not a release, and leaves product `Version` unchanged.
Use Go 1.27.3 and cached dependencies. Packaging also uses POSIX shell tools,
`tar`, `gzip`, and `shasum`; these are not application runtime requirements.

Run from the source root. The fresh temporary output directory avoids replacing
owner files and stays outside the tracked tree:

```sh
set -eu
output=$(mktemp -d "${TMPDIR:-/tmp}/wsup-package.XXXXXX")
version=$(go run ./cmd/wsup --version)
name="wsup_${version}-local_darwin_arm64"
package="$output/$name"
mkdir "$package"
mkdir "$package/licenses"
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local \
  go build -trimpath -buildvcs=false -o "$package/wsup" ./cmd/wsup
cp LICENSE "$package/LICENSE"
cp docs/native-install.md "$package/README.md"
goroot=$(go env GOROOT)
cp "$goroot/LICENSE" "$package/licenses/go-LICENSE"
cp "$goroot/PATENTS" "$package/licenses/go-PATENTS"
for component in crypto net sys text; do
  cp "$goroot/src/vendor/golang.org/x/$component/LICENSE" \
    "$package/licenses/go-x-$component-LICENSE"
  cp "$goroot/src/vendor/golang.org/x/$component/PATENTS" \
    "$package/licenses/go-x-$component-PATENTS"
done
cobra=$(go list -m -f '{{.Dir}}' github.com/spf13/cobra)
pflag=$(go list -m -f '{{.Dir}}' github.com/spf13/pflag)
mousetrap=$(go list -m -f '{{.Dir}}' github.com/inconshreveable/mousetrap)
cp "$cobra/LICENSE.txt" "$package/licenses/cobra-LICENSE"
cp "$pflag/LICENSE" "$package/licenses/pflag-LICENSE"
cp "$mousetrap/LICENSE" "$package/licenses/mousetrap-LICENSE"
{
  printf 'Local candidate, not a published release\nSource: '
  git rev-parse HEAD
  printf '\nWorking-tree status (empty means clean):\n'
  git status --porcelain
  printf '\nToolchain:\n'
  go version
  printf '\nBinary build metadata:\n'
  go version -m "$package/wsup"
} > "$package/BUILD.txt"
chmod 755 "$package/wsup"
tar -czf "$output/$name.tar.gz" -C "$output" "$name"
(cd "$output" && shasum -a 256 "$name.tar.gz" > SHA256SUMS)
(cd "$output" && shasum -a 256 -c SHA256SUMS)
printf 'Local candidate output: %s\n' "$output"
```

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
Exercise help/version, generation, guidance, relocation, collections, update
preview/apply, and explicit checks using a separately supplied test executable.
Confirm reports describe the native runtime and preserve owner customizations.
Never use a real workspace as a packaging fixture.

Keep outputs only as long as needed; remove only the specific disposable files
and directories created for this exercise. A final release must rebuild from
the selected clean commit with its newly chosen version, omit the `-local`
candidate suffix, and repeat all target verification. Local candidates do not
authorize a version bump, tag, installation, signing, or publication.

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
4. Follow the local archive procedure, including required notices and checksums.
   Build a standalone binary outside the tracked tree. Generate a disposable
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

Version 0.2.0 is the native release target under the pre-1.0 minor-version
policy, with its changelog dated 2026-10-25. This date does not establish a tag
or publication: verify the exact clean release commit before separately
authorized tagging or publication.

There is no release automation, package publication, or remote self-update
service. Keep development notes and release-preparation drafts under ignored
`workbench/.local/`; CLI check reports are under ignored `.wsup/reports/`. Publish
user-facing changes and material limitations in the changelog or release notes.
See [AI guidance](ai-guidance.md) for compatibility limits and the
[workspace model](workspace-model.md) for upgrade delivery.
