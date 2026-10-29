---
name: maintain-agents-md
description: Maintain AI.md entry points, rule indexes, portable skills, or optional client adapters. Use when adding domain guidance or troubleshooting instruction discovery.
---

# Maintain agent instructions

1. Read the owner's AI.md, rule index, and existing procedures. Identify the
   behavior, task trigger, and local owner. Never write upward workspace
   relationships or require guidance from outside this workspace.
2. Use entry-point pointers for orientation, rules for constraints, and skills
   for on-demand procedures. Reuse existing sources. Keep client discovery
   separate from portable content.
3. Skills live at `.agents/skills/NAME/SKILL.md`, with matching lowercase
   hyphenated `name` and meaningful nonempty `description` in YAML frontmatter.
   Include invocation cases, ordered actions, and observable completion criteria.
   Link to local contracts rather than duplicating schemas.
4. Route procedures from the owner's rule index. In the product source,
   synchronize canonical skills and distributed variants. Generated local edits are
   user customizations, not changes to the binary's embedded templates.
5. Run `wsup guidance`, or `wsup guidance --project PROJECT_ID`. Check links,
   names, frontmatter, and containment. For starter changes, generate a disposable
   workspace and inspect its guidance with the separately installed binary.
6. Add client adapters only when requested and supported by verified client
   mechanisms. Verify actual loading separately; otherwise provide explicit
   skill paths.

Complete when owner routes resolve and structural checks pass. Report whether
client loading was actually tested. Use a compatible separately installed native
`wsup` from the workspace root, or global `--root` before commands.
