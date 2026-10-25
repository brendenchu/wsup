# Ownership and dependencies

Each workspace knows only itself and its descendants. Documentation, guidance,
registry entries, paths, and commands describe this root and the work it owns.
Do not search outside the root for workspace configuration or instructions,
inherit external workspace policy, or record an upward workspace relationship.
Use explicit local configuration and self-contained required guidance.

Descendant workspaces are independent boundaries, not extensions of this
workspace's policy. Register or operate on them only through explicit local
entries and their own commands; do not inject context about their placement.
Moving a workspace must not require knowledge of where it was generated or kept.

Keep reusable libraries independent of application internals; use public contracts
for host-provided behaviour. Keep executable hosts separate from reusable tooling.
Each owner keeps its tests, dependencies, runtime configuration and required guidance.

Use workspace.json to describe actual project paths, owners and dependencies.
It records intended repository topology, not proof that Git exists at each path.
There is no shared install, build or release unless explicitly configured.

Do not edit installed dependencies or duplicate reusable implementation into
consumers. Use the owner's supported install/release mechanism. Cross-repository
documentation references identify actual dependencies; required instructions
remain self-contained. Framework, container and provider choices are optional.
