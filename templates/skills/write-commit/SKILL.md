---
name: write-commit
description: Draft a concrete Git commit subject from actual staged and unstaged changes. Use for commit-message requests; drafting does not execute Git mutations.
---

# Write a commit message

Identify the owning repository and read its local conventions. Inspect Git status,
staged/unstaged diffs and relevant new files. Treat the existing index as user intent.

Return a concise imperative `type: message` subject for a simple drafting request.
Use style for formatting-only changes, fix for behaviour repairs, and the owner's
other established types as appropriate. Include an issue identifier only when
provided or established by context. A requested body describes resulting behaviour
and only verification actually observed.

For a requested split, account for every included change, keep regression tests
with their fixes and provide exact paths. Explain any index regrouping needed;
all staged work is consumed by a commit. Drafting authorises inspection and printed
commands only: stage, unstage, commit or push only on explicit instruction.
