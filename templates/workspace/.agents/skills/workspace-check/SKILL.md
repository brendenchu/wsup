---
name: workspace-check
description: Run explicitly selected project checks and summarise failures. Use for local CI-style verification, not applying fixes or configuring remote workflows.
---

# Workspace check

Read the local rule index and registry first. Identify projects and inspect their
check argument arrays; configured commands execute with the current user's access.
Use Python 3.9+ and run from the workspace root:

```sh
python3 -B tools/wsup.py validate
python3 -B tools/wsup.py check --project PROJECT_ID
```

Replace PROJECT_ID with a registered ID. Select individual check names with
`--check NAME`; repeat `--project` for multiple owners. The runner validates all
selections before running them, aggregates failures and returns nonzero for failed
or unavailable checks. It does not install dependencies or run dependent projects.

Report each outcome, existing findings and missing prerequisites separately.
A passing command certifies only its configured coverage. Do not apply fixes,
change exclusions or add a workflow just to make a check pass.
