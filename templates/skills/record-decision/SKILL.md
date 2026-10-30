---
name: record-decision
description: Record a workspace-wide decision as a numbered decision record, or supersede an earlier one. Use when the user asks to record, document, or replace a decision, or for an ADR.
---

# Record a decision

1. Read decisions/AGENTS.md, `decisions/TEMPLATE.md`, and the existing records.
   Confirm the decision is workspace-wide; a decision owned by one project
   belongs in that project's documentation. Check for an existing record on the
   same question before writing a new one.
2. Gather the context: the problem, constraints, options considered, and the
   evidence behind them. Link research, issues, or measurements by relative
   path rather than restating them. Ask for the deciders and for anything
   material that only the user knows; never invent consensus or rationale.
3. Number the record one above the highest existing number, zero-padded to
   four digits, with a short hyphenated title: `decisions/0007-short-title.md`.
   Copy the template's structure and fill every section. Date it in the user's
   timezone.
4. Set the status to **Proposed** unless the user says the decision has been
   made; then set **Accepted**. Writing a record does not make a decision, and a
   proposal is never presented as accepted.
5. To supersede a record, write the new one first, citing the old one in its
   Context. Then change only the old record's Status line to
   `Superseded by NNNN` with a relative link to the new record. Never rewrite
   the old record's context, decision, or consequences.
6. In Consequences, state what becomes easier or harder, the follow-up work,
   and the conditions that should prompt revisiting the decision.

Completion means the record follows the template, its status matches what the
user decided, links resolve, and any superseded record points to its
replacement. Recording a decision does not authorise implementing it.
