# Portable AI guidance

The workspace starter provides optional, client-neutral instructions under
`.ai/`. `AI.md` is the canonical entry point; `AGENTS.md` is a short compatibility
pointer. `.ai/rules/index.md` routes tasks to focused rules, and `.ai/skills/`
contains reusable task procedures. Each project starter carries its own
instructions so it remains understandable outside its workspace.

In this generator repository, root `.ai/skills/` is the canonical skill source.
Workspace initialization copies those skills into the generated workspace and
its embedded workspace template. Tests verify the bundled copies stay in sync.

## Domain procedures

The skill kit covers workspace responsibilities alongside general change,
diagnosis, review, and reporting workflows:

| Domain | Procedure |
| --- | --- |
| Project onboarding and Git ownership | [project-onboarding](../.ai/skills/project-onboarding/SKILL.md) |
| Registry, dependencies, checks and settings | [registry-checks](../.ai/skills/registry-checks/SKILL.md) |
| Starter generation and template updates | [template-lifecycle](../.ai/skills/template-lifecycle/SKILL.md) |
| Instructions, skills and client discovery | [ai-guidance-maintenance](../.ai/skills/ai-guidance-maintenance/SKILL.md) |

Both the development and generated workspace rule indexes route to these skills.
Language, framework, and provider procedures belong to the project that needs
them rather than the generic workspace kit.

## Client integration

No agent, model provider, credential, MCP server, or global skill is installed.
Clients differ in how they discover instructions and skills. Configure a chosen
client to read the local `AI.md` and relevant rule index, or provide a skill's
`SKILL.md` path when needed. Add a client-specific adapter only when required and
validate it with that client's current documentation.

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
