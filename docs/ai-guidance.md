# Portable AI guidance

The workspace starter provides optional, client-neutral rules under `guidance/` and
skills under `.agents/skills/`. `AI.md` is the canonical entry point; `AGENTS.md`
is a short compatibility pointer. `guidance/rules/index.md` routes tasks to focused
rules, and `.agents/skills/` contains reusable task procedures. Each project
starter carries its own instructions so it remains understandable outside its
workspace.

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
| Project onboarding and Git ownership | [project-onboarding](../templates/skills/project-onboarding/SKILL.md) |
| Registry, dependencies, checks and settings | [registry-checks](../templates/skills/registry-checks/SKILL.md) |
| Starter generation and template updates | [template-lifecycle](../templates/skills/template-lifecycle/SKILL.md) |
| Instructions, skills and client discovery | [ai-guidance-maintenance](../templates/skills/ai-guidance-maintenance/SKILL.md) |

The generated workspace rule index routes to these skills.
Language, framework, and provider procedures belong to the project that needs
them rather than the generic workspace kit.

## Client integration

No agent, model provider, credential, MCP server, or global skill is installed.
Clients differ in how they discover instructions and skills. Configure a chosen
client to read the local `AI.md` and relevant rule index, or provide a skill's
`SKILL.md` path when needed. Add a client-specific adapter only when required and
validate it with that client's current documentation.

### Skill discovery and compatibility

The compatibility table describes upstream documentation, not certified client
versions. Verify discovery and invocation in the client and repository being
used. WSUP's structural guidance checks do not establish client or OS compatibility.

The canonical shared skill location for fresh workspaces is `.agents/skills/`.
It is a shared location documented by VS Code, Codex, and Gemini CLI, rather than
a location required by the Agent Skills format itself. Structural validation
checks `guidance/rules/index.md`; client discovery and invocation
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
| Claude Code | Project `.claude/skills/`; repository-scoped discovery | Native-location support is documented; shared `.agents/skills/` discovery is not established here. Start a fresh session and inspect available skills when verifying | Explicit-path fallback for WSUP; an optional adapter remains a separate decision |
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

Skill discovery does not establish instruction loading. Keep `AI.md` as the
canonical guidance and the existing `AGENTS.md` pointer; provide explicit paths
when needed. Codex documents
[AGENTS.md discovery](https://developers.openai.com/codex/guides/agents-md);
Gemini documents a separate
[GEMINI.md context mechanism](https://geminicli.com/docs/cli/gemini-md/).
Any additional client entry point must be optional and point to the canonical
guidance instead of copying its policies.

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

WSUP uses shared discovery and explicit-path invocation. It does not provide a
client-specific adapter or installer command.

For setup, open the workspace root and inspect the client's skill list. Provide
the explicit `SKILL.md` path when discovery is unavailable. Refresh client
discovery with its documented controls after a template update. To stop using
skills, use client-level disabling rather than removing files referenced by the
rule index. There is no WSUP adapter to uninstall and no global state to remove.

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
