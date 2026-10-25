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
| Add a project or change Git ownership | [project-onboarding](../../.agents/skills/project-onboarding/SKILL.md) |
| Configure registry, dependencies or check settings | [registry-checks](../../.agents/skills/registry-checks/SKILL.md) |
| Generate starters, maintain templates or update a workspace | [template-lifecycle](../../.agents/skills/template-lifecycle/SKILL.md) |
| Maintain instructions, skills or client discovery | [ai-guidance-maintenance](../../.agents/skills/ai-guidance-maintenance/SKILL.md) |
| Implement or repair code | [repository-change](../../.agents/skills/repository-change/SKILL.md) |
| Diagnose a failure | [diagnose-issue](../../.agents/skills/diagnose-issue/SKILL.md) |
| Review a change | [review-change](../../.agents/skills/review-change/SKILL.md) |
| Plan work | [plan-work](../../.agents/skills/plan-work/SKILL.md) |
| Run workspace checks | [workspace-check](../../.agents/skills/workspace-check/SKILL.md) |
| Infer conventions | [infer-conventions](../../.agents/skills/infer-conventions/SKILL.md) |
| Write a work recap | [recap-work](../../.agents/skills/recap-work/SKILL.md) |
| Draft a commit message | [commit-message](../../.agents/skills/commit-message/SKILL.md) |
