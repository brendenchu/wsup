---
name: onboard-project
description: Onboard a new or existing workspace project. Use when adding or registering a project, choosing Git ownership, or converting a project to an independent repository.
---

# Onboard a project

1. Read the workspace README, [workspace model](../../../docs/workspace-model.md),
   and target project's local guidance. Inspect registry and Git status in both
   owners. Establish path, purpose, owner, kind, dependencies, and intended
   repository ownership before changes.
2. Preserve an existing project's files and setup. For a new starter, inspect
   `wsup init --help` and use `wsup init DESTINATION --template project` at a
   new path. Verify README and resolved containment within this workspace.
3. Follow the workspace model's Git ownership procedure. Independently owned
   projects need an exact root-relative workspace ignore entry, verified with
   `git check-ignore -v`. Obtain approval before removing tracked paths from the
   workspace index or migrating history. Clone/initialize repositories only when
   requested, and verify the project's Git top-level. Never inject an upward
   workspace relationship or project location into its self-contained guidance.
4. Add registry metadata and established dependency IDs. `workspace.json`
   records actual project paths, owners and dependencies; repository ownership
   is intended topology, not proof that Git exists at a path. Register a
   descendant workspace only through an explicit local entry and operate it with
   its own commands; never record where it is placed, so it stays movable. Use
   [run-checks](../run-checks/SKILL.md) for checks and dependency changes.
5. Run `wsup validate` and `wsup projects`. Review both owners' Git status.
   Check project guidance if present. With the `vscode` adapter, preview
   `wsup update` to refresh `<name>.code-workspace` with the new project;
   otherwise generate an editor file only when requested. A hand-edited editor
   file requires deliberate manual review.

Complete when registration validates and Git ownership matches intent. Report
pending repository setup or migration. Use a compatible separately installed
native `wsup` from this workspace root, or supply global `--root` before commands.
