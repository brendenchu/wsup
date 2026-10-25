# Ownership and dependencies

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
