---
name: review-week
description: Write a weekly review from the week's daily journal entries. Use when the user asks for a weekly review, a summary of the week, or what is still open from the week.
---

# Review the week

1. Read journal/AGENTS.md. Establish the ISO week (Monday to Sunday) in the
   user's timezone: the current week unless the user names another. State the
   dates covered.
2. Read only that week's daily entries under `journal/YYYY/MM/`. Do not fill
   gaps from Git history, other folders, or memory; a day without an entry is
   reported as missing, not reconstructed. Read the previous weekly review, if
   any, for threads it left open.
3. Write the review with these sections, citing the day each item comes from:
   - **Progress:** outcomes reached, keeping proposed, uncommitted, committed,
     and deployed work distinct as the entries recorded them.
   - **Decisions:** decisions recorded during the week and who made them.
   - **Open threads:** blockers, unanswered questions, and follow-ups the
     entries left unresolved, including ones carried from the previous review.
   - **Next:** priorities for the coming week, labelled as proposals unless the
     user set them.
4. Save it as `journal/YYYY/YYYY-Www.md`, using the ISO week-year and a
   two-digit week (`journal/2026/2026-W44.md`), titled with the week and its
   dates. If it exists, append an update rather than rewriting it, as for daily
   entries.

Completion means every item cites a daily entry, missing days are stated, and
proposals are labelled. A review does not create commitments, issues, or
messages; turning open threads into work is a separate request.
