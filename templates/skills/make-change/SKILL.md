---
name: make-change
description: Implement or repair code in its owning project while preserving repository ownership and unrelated edits. Use for an authorised implementation task.
---

# Make a change

1. Identify the owning project from the source path (and workspace.json in a
   workspace). Read its README, AI.md and relevant rule index; inspect staged,
   unstaged and untracked work before choosing the edit location.
2. Establish expected behaviour from the task, source and callers. For a defect,
   reproduce it with the narrowest reliable existing test or command.
3. Make a coherent change in the owner. Keep public interfaces and unrelated
   formatting stable unless the task requires otherwise.
4. Run the relevant local checks. Add a regression test when it exercises the
   real behaviour rather than mirroring the implementation.
5. Review the complete diff, including new files, and preserve existing staging.
   Report changed behaviour, verification, limits and outstanding work.

Completion means the authorised outcome is implemented and verified, or a concrete
blocker and completed independent work are reported. Git publication and deployment
remain separate actions requiring explicit authorisation.
