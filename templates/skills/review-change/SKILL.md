---
name: review-change
description: Review a specified branch, commit range or working diff for correctness and repository conventions. Use for review requests, not automatic source edits.
---

# Review a change

Resolve the requested repository and comparison base. Inspect status, staged and
unstaged changes, new files and relevant local rules. Establish what the request
or specification intended; label absent specifications rather than inventing them.

Follow modified behaviour into callers, contracts and tests. For a changed
public API, command, schema, configuration format or persisted record, check
compatibility with existing callers and data, versioning, actionable errors on
invalid input, observable partial failure, and whether migrations or
intentional incompatibilities are stated. Prioritise concrete
regressions, data or permission mistakes, missing coverage and violated owner
boundaries. Distinguish required standards from personal preferences; defer
formatter-owned details to the configured tool.

For each actionable finding, identify the location, triggering conditions,
consequence and supporting evidence. Label uncertainty and avoid counting the
same root cause repeatedly. Separate specification mismatches from coding-rule
violations. If no findings are supported, say so and state verification limits.

Review is read-only unless fixes are separately requested. Do not stage, commit,
publish comments remotely or alter source as part of drafting the review.
