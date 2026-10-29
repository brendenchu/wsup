---
name: maintain-agents-md
description: Maintain AGENTS.md files, portable skills, and agent adapters. Use when adding or changing instructions, skills, or folder rules, or troubleshooting instruction discovery.
---

# Maintain agent instructions

1. Read the root AGENTS.md, the AGENTS.md nearest the affected path, and the
   existing skills. Identify the behaviour, task trigger, and local owner.
   Never write upward workspace relationships or require guidance from outside
   this workspace.
2. Keep each fact in one place. The root AGENTS.md is always loaded, so keep it
   short; folder rules go in that folder's AGENTS.md; on-demand procedures are
   skills. Agent-specific files such as CLAUDE.md only point to AGENTS.md and
   never hold competing instructions.
3. Edit only outside the `<!-- wsup:begin NAME -->` / `<!-- wsup:end NAME -->`
   markers; wsup regenerates the text between them. Owner text elsewhere is
   never touched by wsup.
4. Skills live at `.agents/skills/NAME/SKILL.md`, with a matching lowercase
   hyphenated `name` and a meaningful nonempty `description` in the
   frontmatter. Include invocation cases, ordered actions, and observable
   completion criteria. Link to local contracts rather than duplicating them.
   Generated local edits are owner customizations, not changes to the binary's
   embedded templates.
5. Document resulting behaviour, supported commands, prerequisites and material
   limits; distinguish implemented features from proposals and observed checks
   from goals. Use relative links so instructions work after a move, keep
   credentials, personal data and machine-specific identities out of examples,
   and verify references and examples when they change.
6. Run `wsup guidance`, or `wsup guidance --project PROJECT_ID`. Check links,
   names, frontmatter, markers, and containment. For starter changes, generate
   a disposable workspace and inspect its guidance with the separately
   installed binary.

Complete when references resolve and structural checks pass. Report whether
client loading was actually tested. Use a compatible separately installed native
`wsup` from the workspace root, or global `--root` before commands.
