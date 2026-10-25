# Development guidance

This repository contains the workspace CLI, its tests, and the files copied into
generated workspaces. Keep the starter generic and functional without access to
this checkout.

Each workspace knows only itself and its descendants. Keep documentation,
guidance, registry entries, and commands rooted here; do not discover, inherit,
or reference workspace context outside this root. Descendant workspaces carry
their own self-contained instructions and remain independently usable.

This file is the repository-specific entry point. The root
[rule index](guidance/rules/index.md) routes to local rules in `guidance/rules/`.
Root `guidance/rules/` and `.agents/skills/` are canonical for development.
Native starters are complete sources under `templates/`, embedded directly by
`templates/embed.go`; no generated ZIP or installation overlay is used.
During cutover, `src/templates/` remains the separate Python reference catalogue;
do not edit it for native-only changes. Keep shared guidance synchronized and
native installation instructions consistent with the external-binary contract.
A generated workspace uses its own `AI.md`,
`guidance/rules/`, and `.agents/skills/`; it must not depend on this checkout.

Keep development-session logs, implementation plans, and disposable research
artifacts under ignored `workbench/.local/`. The CLI itself stores check reports under
ignored `.wsup/reports/`; keep those product outputs distinct. Public
documentation describes product behavior and use; the changelog records
user-visible changes, not development-session history.

Before changing behavior, inspect the relevant implementation and tests. For
registry, CLI, or template changes, read [the workspace model](docs/workspace-model.md)
and the affected files under `templates/` (or `src/templates/` for Python reference
changes). Preserve project ownership boundaries
and unrelated work. Do not add framework, client, provider, machine-path, or
organization assumptions to the generic starter.

For task procedures, use the matching skill under `.agents/skills/`; the local rule
index links each workflow.

Run the configured project check after behavior changes:

```sh
python3 -B src/wsup.py check --project wsup
```

Native development uses `cmd/wsup/`, `internal/`, and `templates/`; run
`go test -race ./...` and `go vet ./...` as well. Python reference development
still uses `src/wsup.py`; Python-generated workspaces use `.wsup/wsup.py`.
Native workspaces use a compatible external binary and retain state/reports only.
Never edit generated `.wsup/` output to implement source changes.

Review generated output when changing initialization or update behavior. Do not
commit, publish, deploy, or configure external services unless explicitly asked.
