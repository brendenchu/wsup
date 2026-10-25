# Versioning and releases

## One product version

WSUP uses one version for its CLI, bundled workspace/project templates, and
portable skills. The authoritative value is `VERSION` in
[.wsup/wsup_core/__init__.py](../.wsup/wsup_core/__init__.py). It supplies CLI
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
2. Choose the version using the policy above. Update `VERSION` and finalize the
   corresponding [changelog](../CHANGELOG.md) entry, retaining an Unreleased
   section. Describe migrations and material limitations.
3. On the minimum supported Python 3.9 and an available newer Python, verify:

   ```sh
   python3 --version
   python3 -B .wsup/wsup.py --version
   python3 -B .wsup/wsup.py validate
   python3 -B .wsup/wsup.py guidance
   python3 -B .wsup/wsup.py check --project wsup --show-output
   git diff --check
   ```

   A pass on Python older than 3.9 is not a substitute. Record the OS and Python
   version for each run and report unavailable runtimes as missing coverage;
   claim platform and client support only to the extent actually tested.
4. Generate a disposable workspace outside the checkout. Run its own validation
   and guidance, and use its CLI to generate another workspace. For layout or
   update changes, exercise an actual previous-version workspace as well as
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
   validated release commit, create the annotated tag. For example, only if the
   checked commit declares 0.1.0:

   ```sh
   git tag -a v0.1.0 -m "WSUP 0.1.0"
   git show --no-patch v0.1.0
   ```

8. Publish or push only with explicit authorization. Tag creation is local and
   does not publish a release. If pushing is approved, specify the intended
   remote and exact tag rather than pushing every local tag.

There is no release automation, package publication, or remote self-update
service. Keep development notes and release-preparation drafts under ignored
`workbench/.local/`; CLI check reports are under ignored `.wsup/reports/`. Publish
user-facing changes and material limitations in the changelog or release notes.
See [AI guidance](ai-guidance.md) for compatibility limits and the
[workspace model](workspace-model.md) for upgrade delivery.
