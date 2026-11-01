---
name: make-change
description: Implement or repair code in its owning project while preserving repository ownership and unrelated edits. Use for an authorised implementation task.
---

# Make a change

1. Identify the owning project from the source path (and wsup.json in a
   workspace). Read its README and the nearest AGENTS.md; inspect staged,
   unstaged and untracked work before choosing the edit location. Check the
   current/default branch and the owner's PR/protection policy before editing.
   Reuse an appropriate feature branch or create one when the default branch is
   protected or PRs are required. If protection cannot be verified, use a feature
   branch conservatively and report that uncertainty; do not bypass protections.
2. Establish expected behaviour from the task, source and callers. For a defect,
   reproduce it with the narrowest reliable existing test or command.
3. Make a coherent change in the owner. Keep public interfaces and unrelated
   formatting stable unless the task requires otherwise. When changing a public
   API, command, schema, configuration format or persisted record, inspect its
   callers and compatibility expectations; keep machine-readable formats
   versioned where consumers need to tell contracts apart. Handle invalid input
   with an actionable failure, preserve user data and make partial failure
   observable. Treat migrations, deprecations and destructive cleanup as
   explicit scoped work, and state intentional incompatibilities rather than
   claiming automatic portability. Do not build a particular framework, agent or
   OS into a general interface unless that is the purpose of an optional adapter.
4. Run checks that cover the actual change with the owner's configured tools
   and installed versions rather than an assumed API. Add a regression test when
   it exercises the real behaviour rather than mirroring the implementation.
   Distinguish check-only commands from fix commands: a passing formatter does
   not establish lint correctness, runtime behaviour or rendered equivalence,
   and formatter exclusions stay separate where lint coverage remains useful.
   Use synthetic fixtures or isolated services when tests write data; shared and
   production data need separate explicit authorisation.
5. Review the complete diff, including new files, and preserve existing staging.
   Report changed behaviour, verification, limits and outstanding work.

Completion means the authorised outcome is implemented and verified, or a concrete
blocker and completed independent work are reported. Git publication and deployment
remain separate actions requiring explicit authorisation.
