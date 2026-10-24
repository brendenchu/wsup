# Workspace guidance

Read README.md and `.ai/rules/index.md` before planning changes. Load the rules
matching the task and affected paths. Inspect the owning repository's Git status
and preserve unrelated work. A workspace directory is not necessarily one Git
repository; use workspace.json to locate the owner and its local instructions.

This workspace's `AI.md`, `.ai/rules/`, and `.agents/skills/` are its local
guidance sources. Keep them usable without depending on the generator checkout.
Before changing the CLI, registry, or templates, read
[`docs/workspace-model.md`](docs/workspace-model.md) and the relevant starter
guidance.

Each independent project carries its own required guidance, tests and dependencies.
Use project-local commands and installed versions. Workspace check commands delegate
to the registry; inspect their configured arguments before executing them.

For task procedures, use the skill linked from `.ai/rules/index.md`. Read a skill
only when that task applies. Client discovery support varies; explicit paths work
without installing a client adapter.

Before handoff, report the result, actual checks, remaining limitations and whether
changes are uncommitted. Follow the authorisation boundaries in the change rules.
