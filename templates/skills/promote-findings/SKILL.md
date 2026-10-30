---
name: promote-findings
description: Promote reviewed findings or drafts into reusable reference knowledge or a project's documentation, keeping their evidence. Use when the user asks to promote, publish internally, or move findings into reference or docs.
---

# Promote findings

1. Read the source material and its evidence, and the AGENTS.md of the
   destination. Confirm the user has reviewed the findings: promotion records
   accepted knowledge, it does not review it. Ask which findings to promote
   when the source holds more than one.
2. Choose one destination. Knowledge reusable across the workspace goes to
   `reference/<short-slug>.md` when the reference collection is installed;
   knowledge about one project goes to that project's documentation. Without
   the reference collection, offer only project documentation and suggest
   `wsup add reference`; never create `reference/` yourself.
3. Check the destination for an existing entry on the same subject. Extend or
   correct it rather than creating a competing copy, and keep differences in
   applicability rather than merging them away.
4. Write the entry: the claim or guidance, the evidence with relative links to
   the original, where it applies and where it does not, and the review date.
   Keep the wording of findings that were scoped or uncertain; promotion does
   not make them universal.
5. Add a line to the original pointing to the promoted copy. Leave the original
   in place: removing or archiving it is a separate request.

Completion means the destination holds one canonical entry with its evidence
and applicability, both copies link to each other, and links resolve.
Promotion does not authorise publishing outside the workspace.
