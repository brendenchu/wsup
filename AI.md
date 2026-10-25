# Development guidance

This repository contains the workspace CLI, its tests, and the files copied into
generated workspaces. Keep the starter generic and functional without access to
this checkout.

Each workspace knows only itself and its descendants. Keep documentation,
guidance, registry entries, and commands rooted here; do not discover, inherit,
or reference workspace context outside this root. Descendant workspaces carry
their own self-contained instructions and remain independently usable.

This file is the repository-specific entry point. The root
[rule index](guidance/rules/index.md) routes to focused rules; shared rule text lives
in `templates/workspace/guidance/rules/`. Root `.agents/skills/` is canonical for
skills, and the copies under `templates/workspace/` are generated-workspace
bundles that must stay in sync. A generated workspace uses its own `AI.md`,
`guidance/rules/`, and `.agents/skills/`; it must not depend on this checkout.

Keep development-session logs, implementation plans, and disposable research
artifacts under ignored `.work/`. The CLI itself stores check reports under
ignored `.wsup/reports/`; keep those product outputs distinct. Public
documentation describes product behavior and use; the changelog records
user-visible changes, not development-session history.

Before changing behavior, inspect the relevant implementation and tests. For
registry, CLI, or template changes, read [the workspace model](docs/workspace-model.md)
and the affected files under `templates/`. Preserve project ownership boundaries
and unrelated work. Do not add framework, client, provider, machine-path, or
organization assumptions to the generic starter.

For task procedures, use the matching skill under `.agents/skills/`; the local rule
index links each workflow.

Run the configured project check after behavior changes:

```sh
python3 -B tools/wsup.py check --project wsup
```

These commands are for this visible source checkout. In an initialized
workspace, the installed CLI is `python3 -B .wsup/wsup.py`.

Review generated output when changing initialization or update behavior. Do not
commit, publish, deploy, or configure external services unless explicitly asked.
