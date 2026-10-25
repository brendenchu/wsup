---
name: template-lifecycle
description: Create workspace or project starters, maintain template content, or preview and apply workspace updates. Use for init, update, and generated-workspace portability.
---

# Template lifecycle

1. Read the [workspace model](../../../docs/workspace-model.md) and affected
   guidance. Distinguish new generation, source-template maintenance, and updates.
   Templates are embedded in the separately installed native binary; workspaces
   carry state/reports only, not an editable catalogue or runtime.
2. For generation, use `wsup init DESTINATION`, adding `--template project` for a
   project. Choose a new destination. Inspect guidance, registry where applicable,
   state, and local links. Relative destinations use cwd, not `--root`.
   Collection selection is not yet available.
3. Maintain product starters in their trusted owning source checkout, not
   generated output. Synchronize canonical rules/skills with their distributed
   variants; keep native-specific installation instructions consistent. Keep
   framework setup with its project. Local workspace edits are not generation
   defaults for another workspace.
4. Native CLI updates and Python migration are not yet available. Do not use
   Python to update schema-2 state or change its version to force compatibility.
   For future updates, preserve a reviewed matched backup of files/state and
   inspect Git status, preview, and conflicts before authorized apply. Preserve
   customizations, missing files, and user-owned registry content.
5. Verify source changes in a disposable destination outside the checkout.
   Run `wsup --root DESTINATION validate` and `guidance` for workspaces; inspect
   project guidance separately. Relocate output and generate another workspace
   using the external binary with no checkout/runtime alongside it. Check both
   safe changes and preservation cases.

Complete when output is inspected, portable with a compatible installed binary,
and existing owner bytes are preserved. Report failures and partial destinations;
clean up only disposable paths created for verification. Never recursively remove
partial destinations containing possible concurrent owner content.
