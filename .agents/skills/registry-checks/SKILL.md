---
name: registry-checks
description: Configure workspace registry entries, dependencies, check commands, or local check settings. Use when editing workspace.json, wsup.json, or wsup.local.json.
---

# Registry and checks

1. Read the [workspace model](../../../docs/workspace-model.md), registry, and
   affected projects' local instructions. Establish whether the task changes
   project metadata, dependency edges, check definitions, or execution settings.
   Preserve unrelated entries and existing local overrides.
2. For registry edits, verify each affected path and README exist inside the
   workspace. Use unique IDs and paths, resolve dependency IDs, and check for
   cycles. Set repository ownership as intent, not as proof of Git setup; use
   [project-onboarding](../project-onboarding/SKILL.md) for ownership changes.
3. For checks, inspect the project's existing commands and required executables.
   Encode literal argument arrays launched from the project directory; use
   `{python}` for the CLI's interpreter. Choose required or report mode according
   to the requested failure policy. Specify finding exit codes deliberately;
   preserve genuine command failures. Give disabled checks a concrete reason.
4. For settings, choose shared or local configuration deliberately. Use the
   model's precedence and bounds, keep reports inside `.work/`, and keep local
   settings ignored. Inspect `python3 -B tools/wsup.py config` to verify effective
   values and their sources without publishing machine-specific configuration.
5. Run `python3 -B tools/wsup.py validate` before executing project code. Review
   the selected command arguments, then use
   [workspace-check](../workspace-check/SKILL.md) to run only affected checks.
   Dependencies are registry relationships, not automatic check selection.

Complete when registry validation passes and the affected checks or effective
settings have been verified. Report skipped executions and unresolved tool
requirements; a valid registry alone does not prove a check works.
