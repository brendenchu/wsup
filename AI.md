# Development guidance

This repository contains the workspace CLI, its tests, and the files copied into
generated workspaces. Keep the starter generic and functional without access to
this checkout.

Before changing behavior, inspect the relevant implementation and tests. For
registry, CLI, or template changes, read [the workspace model](docs/workspace-model.md)
and the affected files under `templates/`. Preserve project ownership boundaries
and unrelated work. Do not add framework, client, provider, machine-path, or
organization assumptions to the generic starter.

For task procedures, use the matching skill under `.ai/skills/`; the local rule
index links each workflow. Root skills are canonical and are copied into generated
workspaces.

Run the configured project check after behavior changes:

```sh
python3 -B tools/wsup.py check --project wsup
```

Review generated output when changing initialization or update behavior. Do not
commit, publish, deploy, or configure external services unless explicitly asked.
