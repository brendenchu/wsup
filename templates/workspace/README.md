# Workspace

Use this folder to organize related projects. Projects may share one repository
or keep separate repositories, dependencies, tests, and release cycles. The
registry records project ownership; it does not create a shared build.

## Start

Requires Python 3.11 or newer. From this directory:

```sh
python3 -B tools/wsup.py validate
python3 -B tools/wsup.py projects
python3 -B tools/wsup.py guidance
python3 -B tools/wsup.py editor
```

Add each project to `workspace.json` with a unique ID, existing directory,
kind, repository ownership, purpose, owner, and optional dependencies and checks.
Project directories need a `README.md` and own their setup, dependencies, tests,
and required instructions. The [workspace model](docs/workspace-model.md)
describes the registry, checks, settings, reports, and template updates.

For independent project repositories, add each project's directory to the
workspace `.gitignore` and mark it `independent` in the registry. Workspace-owned
projects stay tracked normally. See [Git ownership](docs/workspace-model.md)
for setup and existing-file precautions.

Run selected configured checks with:

```sh
python3 -B tools/wsup.py check --project PROJECT_ID
```

Review commands before running them: they execute with your permissions. The CLI
does not install dependencies on its own or automatically check dependencies.
Reports are local under ignored `.work/`.

`editor` is an optional VS Code integration. `update` previews template updates;
`update --apply` applies changes only when there are no conflicts.

## Guidance

Read [AI.md](AI.md) and `.ai/rules/index.md` for local instructions. No AI client,
model provider, credential, MCP server, or global configuration is required.
Client integrations are optional and must use the client's supported mechanism.
Portable skills live in `.agents/skills/`. Clients that support that shared
location need no skill installation step; otherwise provide the skill's explicit
`SKILL.md` path. Discovery and actual invocation must be verified separately.
