---
name: investigate-question
description: Investigate a workspace-wide question from scoped sources to a synthesis with its limits. Use when the user asks to research, investigate, compare evidence on, or fact-check something.
---

# Investigate a question

1. Read research/AGENTS.md when `research/` exists. Confirm the question is
   workspace-wide; a question about one project belongs with that project.
   Restate it as a bounded question with what is in and out of scope and what
   answer would be enough to stop. Ask when the scope materially changes the
   answer.
2. Find sources that address the question. Prefer primary sources: official
   documentation, specifications, source code, data, and first-hand reports.
   Use web or connected tools only when they are available, and say which
   sources were and were not reachable. Record for each source why it is
   relevant, its author or publisher, date or version, and how credible it is
   for this specific claim.
3. Map each claim to the sources that support or contradict it, with the
   strength of that support. Keep conflicting evidence visible and name what
   evidence would decide between competing explanations. Never resolve a
   conflict by omission.
4. Write the synthesis: a direct answer, the findings with their sources,
   assumptions, limits, and open questions. Separate observed facts from
   inference, and avoid precision the evidence does not support.
5. With the research collection installed, write it to
   `research/<short-slug>/README.md`, keeping raw captures in ignored
   `workbench/.local/`. Without it, return the synthesis in the conversation
   and suggest `wsup add research`; never create `research/` yourself.

Completion means every claim cites a source or is labelled as inference, gaps
and unreachable sources are stated, and the stopping condition is met or the
remaining work is named. Findings are not decisions: adopting a conclusion,
changing code, or publishing it are separate requests. To move reviewed
findings into reference or project docs, use
[promote-findings](../promote-findings/SKILL.md).
