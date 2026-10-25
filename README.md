# WSUP

A small, local-first foundation for organizing projects that may live in one or
many repositories. It provides a project registry, optional VS Code workspace
generation, explicit local checks, and a portable workspace/project starter.
WSUP supports AI and non-AI workflows without requiring a particular provider
or tool. It favors open, widely supported formats and interfaces, while keeping
service-specific integrations optional. The core should remain useful on its
own, and integrations should be replaceable so people can use the tools that
work for them without making those tools requirements for everyone. The CLI has
no third-party runtime dependencies or required AI service.

**Workspace independence:** Each workspace knows only itself and its descendants.
Its documentation, guidance, registry, and commands are rooted locally.
Descendant workspaces carry their own self-contained instructions; no workspace
context is discovered or inherited from outside the current root.

## Requirements

Use a trusted native `wsup` binary compatible with your machine. Normal
initialization, inspection, collections, and updates need no development toolchain.
Project checks require their configured tools and pass arguments literally
without substitution. WSUP never installs those tools automatically.

Runtime verification is currently macOS ARM64. Unix checks use process-group
supervision; non-Unix checks explicitly refuse launch pending equivalent
descendant cleanup. Cross-compilation does not establish runtime support.

Build from source with Go 1.27.3, pinned in `mise.toml`; see
[Go development](CONTRIBUTING.md#go-development). Development tests also require
Git for ownership and reporting assertions. Only schema-2 workspace state is
supported; unsupported formats fail without writes. No binary installer or published native release
is provided yet; source build does not require a PATH change.

## Try it

Run these from this directory:

```sh
go run ./cmd/wsup validate
go run ./cmd/wsup projects
go run ./cmd/wsup check --project wsup --show-output
```

Create a workspace or a project starter at a new path:

```sh
go run ./cmd/wsup init /path/to/workspace
go run ./cmd/wsup init /path/to/project --template project
```

`init` never overwrites an existing destination. A compiled executable embeds
all templates and works without this checkout. Build outside the tracked tree
with `go build -o /path/to/wsup ./cmd/wsup`, then invoke `/path/to/wsup` (or
`wsup` if separately made available on PATH). Workspaces contain state/reports
under `.wsup/`, not an executable, runtime, or copied catalogue. Relocation
requires a compatible external binary. Shared rules live in `guidance/rules/`
and portable skills remain in `.agents/skills/`.
Generated workspaces include a focused set of development homes, with optional
collections selected explicitly; see the workspace layout below.
The project starter contains project documentation, guidance, ignore rules, and
template state, but no bundled CLI or workspace registry.

## Use the CLI

Run `go run ./cmd/wsup --help` in this checkout, or `wsup --help` with a
separately built executable. Examples below use the latter. The main commands:

| Command | Purpose |
| --- | --- |
| `validate` | Validate `workspace.json` without running project code |
| `projects` | List registered projects |
| `config` | Show effective settings and their sources |
| `guidance` | Check workspace instructions and local links |
| `guidance --project ID` | Check one project's instructions |
| `editor` | Create a relative-path VS Code workspace file |
| `check --project ID` | Run only that project's configured checks |
| `collections` | List available optional collection scaffolds |
| `collections --add NAME` | Preview adding a collection; repeat `--add` to select more |
| `collections --add NAME --apply` | Add scaffolds without replacing existing files |
| `update` | Preview available template updates |
| `update --apply` | Apply conflict-free updates |
| `update --keep PATH` | Retain a same-path customization (repeat for each path) |

Run from the workspace root, or supply the global `--root` option **before**
the command. Executable location does not select the target workspace, and WSUP
does not search parent directories:

```sh
/path/to/wsup --root /path/to/workspace projects
wsup check --help
```

See the [command reference](docs/workspace-model.md#command-reference) for
flags, output formats, exit codes, and root/destination behavior.

Use `--project ID` repeatedly to select more projects and `--check NAME`
repeatedly to choose checks. Review each configured command before running it:
commands execute locally with your permissions. The CLI does not install
dependencies on its own, invoke a shell, or automatically check dependent
projects. Kept customizations retain their existing baselines and remain visible
for review in later update previews.

Check reports are written under the ignored `.wsup/reports/` directory. They
contain results and hashes rather than command output or arguments. Use
`--show-output` to display output in the terminal.

Only schema-2 managed workspaces can update. There is no in-place conversion of
unsupported state. Never edit schema numbers to bypass validation; create a new
destination and explicitly transfer reviewed owner content instead. Before
applying updates, preserve a matched backup of files and state.
See [updates and recovery](docs/workspace-model.md#generated-files-and-updates).

## Workspace layout

Every generated workspace includes `apps/`, `packages/`, `tools/`,
`infrastructure/`, `docs/`, and `workbench/`, alongside the workspace-owned
`guidance/`, `.agents/skills/`, and `.wsup/` foundation. This baseline focuses
on organizing development work, not prescribing a knowledge-management workflow.
`learning/`, `research/`, `reference/`, and `journal/` are CLI-selectable
additions, as are `archive/`, `artifacts/`, and `private/`. Folder
names describe placement, not Git ownership, project kind, sensitivity
controls, backup, or retention guarantees. See the
[workspace model](docs/workspace-model.md#default-layout-and-optional-collections)
for guidance. The template catalogue is embedded in the external binary, never
read from workspace-local overrides. Register projects in
`workspace.json`; projects own their code, tests, dependencies, setup and
required instructions.

Select optional collections during initialization, or add them later:

```sh
wsup init /path/to/workspace --collection archive --collection artifacts
wsup --root /path/to/workspace collections --add private
wsup --root /path/to/workspace collections --add private --apply
```

Each collection creates a purpose-and-boundaries README; journal also includes
an entry template. Existing-workspace additions preview by default; any
conflicting scaffold file blocks the whole apply.
An already-managed scaffold is left untouched, including local customizations.
Selections are recorded in template state so normal updates include them.
They are not inherited when that workspace generates another workspace; select
the desired collections explicitly for each new workspace. Project starters
do not accept collection selections.

These commands do not change ignore rules, permissions, encryption, backup,
sync, indexing, or retention settings. In particular, the private scaffold is
not protected storage.

The registry does not imply a shared Git repository, build, dependency install,
or release process. See the [workspace model](docs/workspace-model.md) for the
registry, settings, checks, reports and update behavior. See [AI guidance](docs/ai-guidance.md)
for the portable instruction kit and optional client integration.

For independent project repositories, add each project's directory to the
workspace `.gitignore` and mark it `independent` in the registry. Workspace-owned
projects stay tracked normally. See [Git ownership](docs/workspace-model.md)
for setup and existing-file precautions.

## This development checkout

The source root separates product development from generated installation output:

| Directory | Role here |
| --- | --- |
| [docs](docs/) | Maintained WSUP product and contributor documentation. |
| [workbench](workbench/) | Reviewed human-authored development drafts, separate from maintained docs and disposable local notes. |
| [cmd/wsup](cmd/wsup/) | Native executable entry point. |
| [internal](internal/) | Go services and colocated regression tests. |
| [templates](templates/) | Complete directly embedded native workspace/project/collection starters. |

The registry describes this repository as one WSUP tool project. Guidance and
portable skills live at the root and match native starter procedures.
Edit native services in `internal/` and starters in `templates/`; rebuilding embeds
starter changes directly. This source repository is not a generated starter and
does not carry managed template state or an installed runtime.
Check reports are created on demand under ignored `.wsup/reports/`.
Use ignored `workbench/.local/` for local notes.

The unused legacy research and journal scaffolds have been retired from this
checkout; their distributable versions remain in the optional catalogue.
No learning, reference, archive, artifacts, or private collection is needed
merely because WSUP offers one. Introduce optional homes here only for actual
development work, with an explicit purpose and owner. An optional `dist/`
directory would hold release bundles, not active workspace tooling.

The unused apps, packages, tools, and infrastructure collection scaffolds are
also absent from this product repository. They remain defaults in
`templates/workspace/`; new generated workspaces retain the full default layout.

## Optional AI skills

Workspace initialization includes 12 portable skills in `.agents/skills/`.
Clients supporting this shared location need no installation command. Open the
workspace root and inspect the client's available skills; otherwise explicitly
ask the assistant to read a skill's `SKILL.md` and follow its procedure.

Run `go run ./cmd/wsup guidance` in development or `wsup guidance`
in a generated workspace to check
local structure and links.
That does not prove a client loaded or invoked a skill. Read
[AI.md](AI.md) and the rule index separately for workspace instructions.
No client adapter, global installation, credential, or model configuration is
created. There is no `skills install` command.

Use the normal template-update preview/apply flow to refresh bundled content;
it does not download a new release. To disable skill use, use the client's
controls rather than deleting canonical files and breaking rule-index links.
See [AI guidance](docs/ai-guidance.md) for the dated compatibility matrix and
[workspace model](docs/workspace-model.md) for safe managed-content updates.

## Contributing and releases

See [CONTRIBUTING.md](CONTRIBUTING.md) for development and verification,
[CHANGELOG.md](CHANGELOG.md) for version milestones, and the
[release guide](docs/releases.md) for versioning and annotated tags.
These are maintainer documents, not generated starter files.

## License

WSUP is licensed under the [MIT License](LICENSE).
When redistributing WSUP code or substantial template content, retain the
copyright and permission notice. Both starters include a `WSUP-LICENSE` notice,
kept in sync with this repository's license. This covers WSUP content and does
not choose a license for your own projects.
