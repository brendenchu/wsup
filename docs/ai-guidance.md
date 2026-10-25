# Portable AI guidance

The workspace starter provides optional, client-neutral rules under `guidance/` and
skills under `.agents/skills/`. `AI.md` is the canonical entry point; `AGENTS.md`
is a short compatibility pointer. `guidance/rules/index.md` routes tasks to focused
rules, and `.agents/skills/` contains reusable task procedures. Each project
starter carries its own instructions so it remains understandable outside its
workspace.

In this generator repository, root `.agents/skills/` is the canonical skill source.
Workspace initialization copies those skills into the generated workspace and
its embedded workspace template. Tests verify the bundled copies stay in sync.
Installed workspace skills are local procedures, not implicit starter overrides.
An installed CLI generates from its bundled catalogue, even when local skills
have been customized.

## Domain procedures

The skill kit covers workspace responsibilities alongside general change,
diagnosis, review, and reporting workflows:

| Domain | Procedure |
| --- | --- |
| Project onboarding and Git ownership | [project-onboarding](../.agents/skills/project-onboarding/SKILL.md) |
| Registry, dependencies, checks and settings | [registry-checks](../.agents/skills/registry-checks/SKILL.md) |
| Starter generation and template updates | [template-lifecycle](../.agents/skills/template-lifecycle/SKILL.md) |
| Instructions, skills and client discovery | [ai-guidance-maintenance](../.agents/skills/ai-guidance-maintenance/SKILL.md) |

Both the development and generated workspace rule indexes route to these skills.
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
a location required by the Agent Skills format itself. Existing workspaces using
`.ai/rules/` or `.ai/skills/` require the explicit pre-1.0 layout migration
described in the [workspace model](workspace-model.md).
The migration relocates only unchanged, manifest-owned content; it blocks on
edited/missing managed files, unknown legacy files, and destination collisions.
Do not copy over customized guidance or fabricate baselines. Structural
validation checks `guidance/rules/index.md`; client discovery and invocation
must still be verified separately.

Installed workspace users run `python3 -B .wsup/wsup.py`; the development
checkout uses `python3 -B tools/wsup.py`. Their bundled template catalogues
produce the same consumer layout. User-edited installed skills are not implicit
starter overrides: initialization uses the installed catalogue.

| Experience | Documented discovery | Precedence and refresh | WSUP evidence |
| --- | --- | --- | --- |
| No AI client | Ordinary CLI commands; no discovery needed | Not applicable | Core commands have no AI-service requirement |
| Any assistant able to read local files | Explicit `SKILL.md` path | User requests the procedure | Portable fallback; invocation not guaranteed |
| VS Code Copilot | `.agents/skills/`, `.github/skills/`, `.claude/skills/` | Same-name skills can appear in multiple locations; inspect the Skills view. Do not infer cross-location precedence from workspace-root precedence | Upstream documentation only |
| Codex CLI / IDE extension | `.agents/skills/` from the current directory up to the repository root | Same-name skills are not merged and can both appear. Changes are detected automatically; restart if needed | Upstream documentation only |
| Gemini CLI | `.agents/skills/` or `.gemini/skills/` | Workspace overrides user scope; `.agents/skills/` wins over `.gemini/skills/` at the same scope. Use `/skills list` and `/skills reload`; activation requests consent | Upstream documentation only |
| Claude Code | Project `.claude/skills/`; parent discovery stops at the repository root | Native-location support is documented; shared `.agents/skills/` discovery is not established here. Start a fresh session and inspect available skills when verifying | Explicit-path fallback for WSUP; an optional adapter remains a separate decision |
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
Repository traversal differs by client; a parent workspace's skills must not be
assumed to load inside an independent child repository. Keep project guidance
with its owner and verify loading in the actual root/session being used.

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
