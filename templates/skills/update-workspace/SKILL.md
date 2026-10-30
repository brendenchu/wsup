---
name: update-workspace
description: Create workspace or project starters, maintain template content, or preview and apply workspace updates. Use for init, update, and generated-workspace portability.
---

# Update the workspace

1. Read the [workspace model](../../../docs/workspace-model.md) and affected
   guidance. Distinguish new generation, source-template maintenance, and updates.
   Templates are embedded in the separately installed native binary; workspaces
   carry `wsup.lock` and reports only, not an editable catalogue or runtime.
2. For generation, use `wsup init DESTINATION --yes` (or explicit `--preset`,
   `--agent`, and `--editor` flags) so init never waits on its terminal
   questions, adding `--template project` for a project. Choose a new
   destination. Inspect guidance, `wsup.json` where applicable,
   `wsup.lock`, and local links. Relative destinations use cwd, not `--root`.
   Use repeatable `--collection NAME` only for explicit optional workspace homes.
   `wsup add NAME` previews adding a collection or adapter to an existing workspace; apply
   only when authorized and conflict-free with `--apply`. Never adopt pre-existing
   unmanaged scaffold files, even if identical, or assume parent selections carry
   into child generation. Selection creates no security/backup/retention controls.
3. Maintain product starters in their trusted owning source checkout, not
   generated output. Synchronize canonical rules/skills with their distributed
   variants; keep native-specific installation instructions consistent. Keep
   framework setup with its project. Local workspace edits are not generation
   defaults for another workspace.
4. For native updates, preserve a reviewed matched backup of files and `wsup.lock` and
   inspect Git status, then run `wsup update`. Review all actions and conflicts
   before authorized `wsup update --apply`. Report the preview's `modules`:
   offered ids need an explicit `wsup add`, and unlisted modules are not
   removed. Repeat `--keep PATH` only for eligible existing same-path
   customizations; their original baselines remain unchanged.
   Preserve missing files for manual resolution and never regenerate the
   user-owned `wsup.json`. I/O failures may leave partial content with the old lock;
   inspect failures rather than retrying blindly. Restore matched files and lock
   from backup before retrying. No automatic backup or rollback is provided.
   Only lock version 1 is supported; never change version numbers to force
   compatibility. A wsup 0.2.0 starter (`.wsup/template-state.json`) is
   converted with `wsup migrate`: review its preview, actions, and notes, then
   `wsup migrate --apply` only when authorized; report the backup folder and
   every note left for the owner. An owner file at a path wsup installs is a
   conflict; ask whether to keep it (`--keep PATH`) or replace it, never decide
   for the owner.
5. Verify source changes in a disposable destination outside the checkout.
   Run `wsup --root DESTINATION validate` and `guidance` for workspaces; inspect
   project guidance separately. Relocate output and generate another workspace
   using the external binary with no checkout/runtime alongside it. Check both
   safe changes and preservation cases.

Complete when output is inspected, portable with a compatible installed binary,
and existing owner bytes are preserved. Report failures and partial destinations;
clean up only disposable paths created for verification. Never recursively remove
partial destinations containing possible concurrent owner content.
