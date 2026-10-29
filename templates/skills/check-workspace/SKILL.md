---
name: check-workspace
description: Run explicitly selected project checks and summarise failures. Use for local CI-style verification, not applying fixes or configuring remote workflows.
---

# Check the workspace

Read AGENTS.md and the registry. Inspect selected check argv; commands run
with your access and are not sandboxed. Use a compatible separately installed
native `wsup` from the workspace root:

```sh
wsup validate
wsup check --project PROJECT_ID
```

Replace PROJECT_ID with a registered ID. Use `--check NAME` to select check names
and repeat `--project` for multiple owners. All selections validate before
execution. Dependencies are not automatic selections; tools are never installed.
Arguments are passed literally without substitution. Unix check execution is
verified on macOS ARM64; non-Unix execution refuses launch.

Run independent checks even when another fails. Report outcomes, findings,
skips, failures, and missing prerequisites separately, recording the commands
and environmental limits. Never report a skipped or unrun check as passing.
Default output is hidden; `--show-output` prints bounded retained output and may
reveal secrets. Reports omit captured output, arguments, and environment values,
but remain local evidence requiring review before sharing.

A passing check certifies only its configured coverage. Do not apply fixes,
change exclusions, or add workflows just to make it pass. Use global `--root`
before the command to target another explicit workspace.
