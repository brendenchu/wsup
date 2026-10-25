---
name: ai-guidance-maintenance
description: Maintain AI.md entry points, rule indexes, portable skills, or optional client adapters. Use when adding domain guidance or troubleshooting instruction discovery.
---

# AI guidance maintenance

1. Read the owner's AI.md, rule index, and existing procedures. Identify the
   behavior the guidance should change and its task trigger. Choose the scope:
   portable workspace guidance or project-specific requirements owned locally.
2. Use an entry-point pointer for always-relevant orientation, a rule for
   constraints, or a skill for an on-demand procedure. Reuse an existing source
   where it already covers the behavior. Keep client-specific discovery separate
   from portable content.
3. For a skill, create `.agents/skills/NAME/SKILL.md` with matching lowercase,
   hyphenated `name` and a nonempty `description` in YAML frontmatter. Put the
   distinct invocation cases in the description, ordered actions in the body,
   and observable completion criteria at the end. Link to authoritative local
   contracts rather than duplicating their schemas.
4. Route the new procedure from the owner's rule index using a clear task
   trigger and relative link. In the generator repository, maintain canonical
   root skills and matching bundled template copies and routes. In a generated
   workspace, treat local edits as user customizations that updates may flag.
5. Run `python3 -B .wsup/wsup.py guidance` for workspace guidance, or
   `python3 -B .wsup/wsup.py guidance --project PROJECT_ID` for a registered
   project's kit. Check relative links, skill names, frontmatter, and instruction
   boundaries. For starter changes, generate a disposable workspace and run its
   own guidance command to verify distribution.
6. If client auto-discovery is requested, verify the chosen client's supported
   locations and configuration before adding an adapter. Validate actual loading
   separately from structural checks; otherwise provide the explicit skill path.

Complete when the procedure is reachable from its owner, structural checks pass,
and portable copies match where applicable. State whether client loading was
tested; file presence alone does not establish invocation.

Commands above target an installed workspace. In the WSUP development checkout,
use `tools/wsup.py` instead; inspect the owner before choosing a CLI.
