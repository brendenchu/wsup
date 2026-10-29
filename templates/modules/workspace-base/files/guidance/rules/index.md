# Rule index

Each workspace knows only itself and its descendants.
Paths and instruction discovery are scoped to this repository.
Read every row relevant to the task; a path match alone may miss a task rule.

| Trigger / paths | Rule |
| --- | --- |
| Any implementation, configuration or Git operation | [changes.md](changes.md) |
| Workspace scope, adding projects, dependencies or changing ownership | [boundaries.md](boundaries.md) |
| Tests, checks, formatting or static analysis | [verification.md](verification.md) |
| Research, captures, findings or generated evidence | [evidence.md](evidence.md) |
| Public APIs, CLI/config contracts, schemas or migrations | [interfaces.md](interfaces.md) |
| Documentation, instructions or skills | [documentation.md](documentation.md) |
| `journal/**` or a work recap | [journaling.md](journaling.md) |

Project-specific language and framework rules belong in that project's own index.

Use the task procedure that matches the request:

| Task | Skill |
| --- | --- |
| Add a project or change Git ownership | [onboard-project](../../.agents/skills/onboard-project/SKILL.md) |
| Configure registry, dependencies or check settings | [run-checks](../../.agents/skills/run-checks/SKILL.md) |
| Generate starters, maintain templates or update a workspace | [update-workspace](../../.agents/skills/update-workspace/SKILL.md) |
| Maintain instructions, skills or client discovery | [maintain-agents-md](../../.agents/skills/maintain-agents-md/SKILL.md) |
| Plan work | [plan-work](../../.agents/skills/plan-work/SKILL.md) |
| Run workspace checks | [check-workspace](../../.agents/skills/check-workspace/SKILL.md) |
| Write a work recap | [recap-work](../../.agents/skills/recap-work/SKILL.md) |

Project collections (`apps`, `packages`, `tools`, `infra`, `sandbox`,
`templates`, `forks`) add code-change skills under `.agents/skills/` when
selected: `make-change` to implement or repair code, `diagnose-issue` for a
failure, `review-change` for a review, `infer-conventions` to infer
conventions, and `write-commit` to draft a commit message.
