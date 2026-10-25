# Ownership and dependencies

Each workspace knows only itself and its descendants. Keep this repository's
required guidance self-contained; do not discover or inherit workspace context
outside its root. Record ownership and dependencies explicitly in local files.

Keep reusable libraries independent of application internals; use public contracts
for host-provided behaviour. Keep executable hosts separate from reusable tooling.
Each owner keeps its tests, dependencies, runtime configuration and required guidance.

If this repository is a workspace, its own workspace.json describes descendant
paths, owners and dependencies. It records intended repository topology, not proof
that Git exists at each path. Other projects need no workspace registry.
There is no shared install, build or release unless explicitly configured.

Do not edit installed dependencies or duplicate reusable implementation into
consumers. Use the owner's supported install/release mechanism. Cross-repository
documentation references identify actual dependencies; required instructions
remain self-contained. Framework, container and provider choices are optional.
