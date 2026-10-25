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
   Use repeatable `--collection NAME` only for explicit optional workspace homes.
   `wsup collections --add NAME` previews existing-workspace additions; apply
   only when authorized and conflict-free with `--apply`. Never adopt pre-existing
   unmanaged scaffold files, even if identical, or assume parent selections carry
   into child generation. Selection creates no security/backup/retention controls.
3. Maintain product starters in their trusted owning source checkout, not
   generated output. Synchronize canonical rules/skills with their distributed
   variants; keep native-specific installation instructions consistent. Keep
   framework setup with its project. Local workspace edits are not generation
   defaults for another workspace.
4. For native updates, preserve a reviewed matched backup of files/state and
   inspect Git status, then run `wsup update`. Review all actions and conflicts
   before authorized `wsup update --apply`. Repeat `--keep PATH` only for eligible
   existing same-path customizations; their original baselines remain unchanged.
   Preserve missing files for manual resolution and never regenerate the
   user-owned registry. I/O failures may leave partial content with old state;
   inspect failures rather than retrying blindly. Python migration apply is not
   yet available; explicit `update --migrate-native` only previews schema-1
   conversion and retirement. Never delete configuration/reports to bypass its
   conservative blockers, and never combine it with `--apply` while disabled.
   Do not use Python to update schema-2 state or change its version
   to force compatibility.
5. Verify source changes in a disposable destination outside the checkout.
   Run `wsup --root DESTINATION validate` and `guidance` for workspaces; inspect
   project guidance separately. Relocate output and generate another workspace
   using the external binary with no checkout/runtime alongside it. Check both
   safe changes and preservation cases.

Complete when output is inspected, portable with a compatible installed binary,
and existing owner bytes are preserved. Report failures and partial destinations;
clean up only disposable paths created for verification. Never recursively remove
partial destinations containing possible concurrent owner content.
