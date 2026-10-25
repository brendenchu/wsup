# Workspace

Use this folder to organize related projects. Projects may share one repository
or keep separate repositories, dependencies, tests, and release cycles. The
registry records project ownership; it does not create a shared build.

Keep the workspace useful without requiring a particular provider or tool.
Prefer open, widely supported formats and interfaces; keep service-specific
integrations optional and replaceable. Projects can have their own real
requirements, but tools chosen for one project should not become requirements
for the whole workspace.

## Start

Requires Python 3.9 or newer. An existing macOS developer-tools Python 3.9 can
run the CLI; no system Python replacement is needed. WSUP compatibility does
not extend upstream Python security maintenance. From this directory:

```sh
python3 -B .wsup/wsup.py validate
python3 -B .wsup/wsup.py projects
python3 -B .wsup/wsup.py guidance
python3 -B .wsup/wsup.py editor
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
python3 -B .wsup/wsup.py check --project PROJECT_ID
```

Review commands before running them: they execute with your permissions. The CLI
does not install dependencies on its own or automatically check dependencies.
Reports are local under ignored `.wsup/reports/`.

Create project groupings such as `apps/`, `tools/`, or `learning/` only when
needed. Other optional collections may separate durable docs, investigations,
reference, drafts, archived work, or retained deliverables. No empty grouping
directories or journal are required; see the [workspace model](docs/workspace-model.md)
for their boundaries. Keep human-authored
drafts in a visible `workbench/` if useful. Sanitized drafts are eligible for
local Git tracking; review them before staging. Nothing is committed
automatically. Keep restricted records outside documentation, workbench drafts,
and Git repositories. Promote approved knowledge to the owning project's
maintained documentation instead of keeping competing canonical copies. A
`private/` folder or ignore rule is not an access-control boundary.

The bundled runtime and the single installed generation catalogue live under
`.wsup/`. The registry, guidance, and skills remain workspace-owned. To change
the product's starters, edit a trusted source checkout and refresh deliberately;
there is no competing editable root template copy or implicit override tree.

`editor` is an optional VS Code integration. `update` previews template updates;
`update --apply` applies changes only when there are no conflicts.

## Guidance

Read [AI.md](AI.md) and `guidance/rules/index.md` for local instructions. No AI client,
model provider, credential, MCP server, or global configuration is required.
Client integrations are optional and must use the client's supported mechanism.
Portable skills live in `.agents/skills/`. Clients that support that shared
location need no skill installation step; otherwise provide the skill's explicit
`SKILL.md` path. Discovery and actual invocation must be verified separately.

Initialization includes all 12 skills; no adapter or `skills install` command is
needed or provided. Open the workspace root and inspect the client's available
skills, or explicitly ask it to read `.agents/skills/project-onboarding/SKILL.md`
and follow that procedure. `guidance` checks local files and links, not client
loading. Always read AI.md and the rule index separately for instructions.

Use template update preview/apply to refresh bundled content after obtaining a
newer trusted CLI; it does not download releases. Use the client's controls to
disable skill use rather than deleting files linked from the rule index. See the
[workspace model](docs/workspace-model.md) for legacy migration and recovery.
Independent project repositories retain their own guidance; parent skills are
not installed into them automatically.

## WSUP attribution

The bundled WSUP CLI, templates, and guidance are covered by
[WSUP-LICENSE](WSUP-LICENSE). Retain this notice when redistributing WSUP content.
It does not choose a license for your own projects.
