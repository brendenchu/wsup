---
name: recap-work
description: Write an evidence-backed work recap, saved as a journal entry when the journal collection is installed. Use when the user requests a recap or workspace policy explicitly enables routine journaling.
---

# Recap work

Read journal/AGENTS.md and the existing daily entry when `journal/` exists.
Establish the date and timezone from user context; state uncertainty rather than
inventing timestamps. Inspect only relevant owner histories and observed work.

Cover material outcomes, decisions, actual verification, blockers and useful
follow-ups. Attribute user decisions, Git authorship and assistant execution
separately, and label unknown attribution; task delegation does not establish
personal authorship. Separate proposed, uncommitted, committed and deployed
work. Link canonical evidence with its owner and exclude secrets. Keep updates
concise and avoid duplicate entries.

With the journal collection installed, append one timestamped update to
`journal/YYYY/MM/YYYY-MM-DD.md` (ISO timestamps with an explicit offset),
following `journal/TEMPLATE.md`, and correct earlier claims by appending rather
than rewriting them. Without it, return the recap in the conversation and
suggest `wsup collections --add journal`; never create `journal/` yourself.

Completion means claims have evidence, proposed/uncommitted work is labelled and
sensitive data is omitted. A recap does not authorise code changes, commits,
external messages, publication or background collection. Routine journaling is
an explicit workspace policy, not implied by installing this skill.
