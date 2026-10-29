---
name: run-checks
description: Configure workspace registry entries, dependencies, check commands, or local check settings. Use when editing workspace.json, wsup.json, or wsup.local.json.
---

# Run checks

1. Read the [workspace model](../../../docs/workspace-model.md), registry, and
   affected projects' instructions. Identify metadata, dependency, check, or
   execution-setting changes. Preserve unrelated entries and local overrides.
2. Verify affected project directories and READMEs exist within the workspace.
   Use unique IDs/paths, resolve dependencies, reject cycles. Repository metadata
   describes intent, not proof of Git setup; use
   [onboard-project](../onboard-project/SKILL.md) for ownership changes.
3. Inspect project commands and required executables. Encode literal argv
   launched in the project directory. Executables must be available by explicit
   path or on PATH; missing tools are launch errors. Arguments are not substituted.
   Choose required/report policy and finding exit codes deliberately, preserving
   genuine failures. Disabled checks need concrete reasons.
4. Select shared/local settings deliberately. Follow documented precedence and
   bounds; keep reports within `.wsup/reports/` and local settings ignored.
   Inspect `wsup config` for effective values/source labels without publishing
   machine-specific configuration.
5. Run `wsup validate` before project code. Review selected arguments, then use
   [check-workspace](../check-workspace/SKILL.md) for explicitly affected checks.
   Dependencies do not imply automatic execution.

Complete when validation and affected checks/settings are verified. Report
skips and missing tools separately; valid configuration does not prove execution.
Use a compatible separately installed native `wsup` from this workspace root,
or supply global `--root` before commands.
