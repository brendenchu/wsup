# Portable AI guidance

Generated workspaces and projects keep their agent instructions in `AGENTS.md`
and reusable task procedures in `.agents/skills/`. The root `AGENTS.md` is short
and always loaded: it holds the workspace rules and generated Layout and Skills
sections between `<!-- wsup:begin NAME -->` / `<!-- wsup:end NAME -->` markers.
Each collection folder has its own `AGENTS.md` with that folder's rules. Each
project starter carries its own instructions so it remains understandable
outside its workspace.

In this generator repository, `templates/skills/` is the only
skill source; it is embedded directly in the binary.
Installed workspace skills are local procedures, not implicit starter overrides.
An installed CLI generates from its embedded catalogue, even when local skills
have been customized.

## Domain procedures

The skill kit covers workspace responsibilities alongside general change,
diagnosis, review, and reporting workflows:

| Domain | Procedure |
| --- | --- |
| Project onboarding and Git ownership | [onboard-project](../templates/skills/onboard-project/SKILL.md) |
| Registry, dependencies, checks and settings | [run-checks](../templates/skills/run-checks/SKILL.md) |
| Starter generation and template updates | [update-workspace](../templates/skills/update-workspace/SKILL.md) |
| Instructions, skills and client discovery | [maintain-agents-md](../templates/skills/maintain-agents-md/SKILL.md) |

These are base skills, so every workspace has them and its AGENTS.md links to
them. The code-change skills (make-change, review-change, diagnose-issue,
write-commit, infer-conventions) are owned by the project collections and
installed only when one is selected; the project starter always has them.
Which modules install which skills is declared in each `module.json`.
Language, framework, and provider procedures belong to the project that needs
them rather than the generic workspace kit.

## Client integration

No model provider, credential, MCP server, or global skill is installed.
Clients differ in how they discover instructions and skills, so `init` adds
agent adapters for clients that do not read `AGENTS.md` and `.agents/skills/`
directly. An adapter holds only pointers, never instructions:

| Adapter | Files | Why |
| --- | --- | --- |
| `claude` | `CLAUDE.md` (`@AGENTS.md`), `.claude/skills -> ../.agents/skills` | Claude Code reads `CLAUDE.md` and `.claude/skills/` |
| `gemini` | `GEMINI.md` (`@AGENTS.md`) | Gemini CLI reads `GEMINI.md`; it already finds `.agents/skills/` |

All adapters are installed by default; `--agent NAME` (repeatable) selects
some and `--agent none` skips them. `.claude/skills` is the only symlink wsup
creates. Clients that read `AGENTS.md` natively, such as Codex, need no
adapter. File presence does not prove a client loads them; verify in the client.

### Skill discovery and compatibility

The compatibility table describes upstream documentation, not certified client
versions. Verify discovery and invocation in the client and repository being
used. WSUP's structural guidance checks do not establish client or OS compatibility.

The canonical shared skill location for fresh workspaces is `.agents/skills/`.
It is a shared location documented by VS Code, Codex, and Gemini CLI, rather than
a location required by the Agent Skills format itself. Structural validation
checks `AGENTS.md` files, their markers, and skills; client discovery and invocation
must still be verified separately.

Development runs `go run ./cmd/wsup`; generated workspaces use a compatible
external native `wsup` binary. Product sources live in `cmd/`, `internal/`, and
`templates/`; `.wsup/` holds managed state/reports in native workspaces.
User-edited workspace skills are not implicit starter overrides: initialization
always uses the invoked binary's catalogue.

| Experience | Documented discovery | Precedence and refresh | WSUP evidence |
| --- | --- | --- | --- |
| No AI client | Ordinary CLI commands; no discovery needed | Not applicable | Core commands have no AI-service requirement |
| Any assistant able to read local files | Explicit `SKILL.md` path | User requests the procedure | Portable fallback; invocation not guaranteed |
| VS Code Copilot | `.agents/skills/`, `.github/skills/`, `.claude/skills/` | Same-name skills can appear in multiple locations; inspect the Skills view. Do not infer cross-location precedence from workspace-root precedence | Upstream documentation only |
| Codex CLI / IDE extension | `.agents/skills/` from the current directory up to the repository root | Same-name skills are not merged and can both appear. Changes are detected automatically; restart if needed | Upstream documentation only |
| Gemini CLI | `.agents/skills/` or `.gemini/skills/` | Workspace overrides user scope; `.agents/skills/` wins over `.gemini/skills/` at the same scope. Use `/skills list` and `/skills reload`; activation requests consent | Upstream documentation only |
| Claude Code | Project `.claude/skills/`; repository-scoped discovery | Native-location support is documented; the `claude` adapter links `.claude/skills` to `.agents/skills` rather than relying on shared-path discovery. Start a fresh session and inspect available skills when verifying | Installed by default with the `claude` adapter; explicit paths remain a fallback |
| Copilot CLI, cloud agent, app, other assistants | Must be checked for each experience | VS Code settings are not automatically inherited by other clients | No automatic-discovery claim yet |

Sources:

- [Agent Skills specification](https://agentskills.io/specification): portable
  folder format with required name/description metadata and optional resources.
- [VS Code skill discovery](https://code.visualstudio.com/docs/agent-customization/agent-skills):
  supported locations, duplicate visibility, and custom-location limitations.
- [Codex skills](https://developers.openai.com/codex/skills): repository traversal,
  duplicate names, and reload behavior.
- [Gemini CLI skills](https://geminicli.com/docs/cli/skills/): shared-path alias,
  precedence, activation consent, listing, and reload.
- [Claude Code skills](https://code.claude.com/docs/en/skills): native project
  locations and repository-boundary traversal.

Skill discovery does not establish instruction loading. `AGENTS.md` is the
canonical guidance; provide explicit paths when needed. Codex documents
[AGENTS.md discovery](https://developers.openai.com/codex/guides/agents-md);
Gemini documents a separate
[GEMINI.md context mechanism](https://geminicli.com/docs/cli/gemini-md/).
Each adapter entry point imports the canonical `AGENTS.md` instead of copying
its policies.

Workspace-local discovery is the default. WSUP will not install skills into
independent child repositories, user home directories, or machine-wide locations.
Each workspace knows only itself and its descendants. Repository traversal
differs by client, but it does not grant instruction inheritance: use the
current root's own guidance and skills. Keep descendant guidance self-contained
and verify loading in the actual root/session being used. WSUP guidance does
not configure or enforce a client's discovery boundary; inspect the client's
controls separately if it loads context outside the intended root.

No provider selection, credentials, client installation, network installer, or
marketplace is required. Clients supporting the shared location need no
per-client skill installer.

### Setup and client controls

WSUP uses shared discovery, the adapters above, and explicit-path invocation.
It has no installer command.

For setup, open the workspace root and inspect the client's skill list. Provide
the explicit `SKILL.md` path when discovery is unavailable. Refresh client
discovery with its documented controls after a template update. To stop using
skills, use client-level disabling rather than removing files referenced by the
AGENTS.md skills table. Removing an adapter is not supported yet: its files and
link are recorded in template state, so `update` reports deleted adapter files
and re-creates the link. There is no global state to remove.

## Keep guidance useful

- Keep always-read instructions short and put task-specific detail behind clear
  links.
- Give each rule or procedure one authoritative source.
- Add language, framework, and service requirements only to the project that
  owns them.
- Keep starter instructions local and usable when a project is moved.
- Treat generated guidance as documentation, not a guarantee that a client loads
  or follows it.

The starter's task procedures are optional. Journaling is a local policy, not an
automatic collection or publication feature.
