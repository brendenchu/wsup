---
name: project-onboarding
description: Onboard a new or existing workspace project. Use when adding or registering a project, choosing Git ownership, or converting a project to an independent repository.
---

# Project onboarding

1. Read the workspace README, [workspace model](../../../docs/workspace-model.md),
   and target project's local guidance. Inspect the registry and Git status in
   the workspace and project. Establish the project's path, purpose, owner,
   kind, dependencies, and intended repository ownership before making changes.
2. Choose the existing-project or new-project path. For an existing project,
   preserve its files and setup. For a new starter, inspect
   `python3 -B .wsup/wsup.py init --help` and generate with `--template project`
   into a new destination. Confirm the directory contains a README and remains
   inside the workspace, including through symlinks.
3. Apply the Git ownership procedure in the workspace model. For independent
   ownership, add the exact root-relative project directory to the workspace
   ignore file and verify it with `git check-ignore -v`. Check whether this workspace
   already tracks project files; obtain approval for index removal or history
   migration before converting them. Initialize or clone a repository only when
   requested, and verify its top-level directory is the project itself.
   Keep the project's guidance self-contained; never write its location within
   this workspace or an upward workspace relationship into its required docs.
4. Add the project's registry entry with the established ownership and dependency
   IDs. Reuse the [registry-checks](../registry-checks/SKILL.md) procedure when
   adding checks or changing dependencies.
5. Run `python3 -B .wsup/wsup.py validate` and `python3 -B .wsup/wsup.py projects`.
   Review Git status in both owners: workspace metadata belongs to the workspace,
   independent project contents belong to the project. Run project guidance
   validation if it carries the instruction kit. Generate an editor file only
   when requested; an existing editor file requires deliberate manual review.

Complete when the project is registered, validation passes, and Git ownership
matches intent. Report pending repository setup or migration explicitly.

Commands above target generated workspaces. In the WSUP development checkout,
use `src/wsup.py` instead. Run commands from the owning workspace root.
