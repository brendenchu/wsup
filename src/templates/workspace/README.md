# Workspace

Use this folder to organize related projects. Projects may share one repository
or keep separate repositories, dependencies, tests, and release cycles. The
registry records project ownership; it does not create a shared build.

Each workspace knows only itself and its descendants. Its documentation,
guidance, registry, and commands are self-contained and scoped to this root.
Descendant workspaces carry their own instructions; no workspace context is
discovered or inherited from outside this root.

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

The CLI uses only the Python standard library; no package installation or
globally installed `wsup` command is required. `editor` is optional and refuses
to overwrite an existing file. The script location does not select the target
root: run from this workspace, or place `--root /path/to/workspace` before the
command. There is no parent-directory discovery.

Use `python3 -B .wsup/wsup.py --help` or `COMMAND --help` for options. See the
[command reference](docs/workspace-model.md#command-reference) for flags,
outputs, exit codes, and source-versus-generated invocation.

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
Without `--check`, all configured checks on the selected project run. Repeat
`--project` or `--check` to select more; every selected check name must exist on
each selected project. Use `--show-output` to print bounded captured output and
`config` to inspect workspace settings.

WSUP creates an opinionated set of working homes in every new workspace:
`apps/`, `packages/`, `tools/`, `infrastructure/`, `docs/`, and `workbench/`,
alongside `guidance/`, `.agents/skills/`, and `.wsup/`. These organize
development work without prescribing a knowledge-management workflow.
They are defaults, not a requirement to populate every directory. See the
[workspace model](docs/workspace-model.md#default-layout-and-optional-collections)
for their distinct purposes.

`learning/`, `research/`, `reference/`, and `journal/` are opt-in, as are
`archive/`, `artifacts/`, and `private/`. Select only homes with a real use;
understand lifecycle, provenance, access, and recovery needs before use. A
`private/` folder or ignore rule is not an access-control boundary. Keep
restricted records outside documentation, workbench drafts, and Git
repositories. Review and sanitize human-authored drafts before staging; nothing
is committed automatically. Promote approved knowledge to the owning project's
maintained documentation instead of keeping competing canonical copies.

Use the CLI to select optional collection scaffolds:

```sh
python3 -B .wsup/wsup.py collections
python3 -B .wsup/wsup.py collections --add archive --add artifacts
python3 -B .wsup/wsup.py collections --add archive --add artifacts --apply
python3 -B .wsup/wsup.py init /path/to/another-workspace --collection archive
```

Available names are `archive`, `artifacts`, `journal`, `learning`, `private`,
`reference`, and `research`. Existing-workspace
additions preview by default and never replace existing files. Each scaffold
creates a README (journal also includes an entry template), not access controls, ignore rules, backups, or retention
settings. Selected scaffolds remain managed by `update`; existing custom
READMEs require manual resolution. A new workspace does not inherit the
generator workspace's choices; select them explicitly with `--collection`.

The bundled runtime and the single installed generation catalogue live under
`.wsup/`. The registry, guidance, and skills remain workspace-owned. To change
the product's starters, edit a trusted source checkout and refresh deliberately;
there is no competing editable root template copy or implicit override tree.

`editor` is an optional VS Code integration. `update` previews template updates;
`update --apply` applies changes only when there are no conflicts.
Obtain a newer trusted CLI separately before updating; this command uses the
invoked CLI's catalogue and does not download releases. Back up local work and
review the full preview before applying. See the
[update and recovery instructions](docs/workspace-model.md#generated-files-and-updates).

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
Independent project repositories retain their own guidance; workspace skills are
not installed into them automatically.

## WSUP attribution

The bundled WSUP CLI, templates, and guidance are covered by
[WSUP-LICENSE](WSUP-LICENSE). Retain this notice when redistributing WSUP content.
It does not choose a license for your own projects.
