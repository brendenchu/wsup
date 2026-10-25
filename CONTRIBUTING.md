# Contributing to WSUP

WSUP is a local-first workspace CLI and starter, not a shared build system or an
AI-service integration. Keep changes generic, portable, and usable without this
checkout.

## Before changing code

Read [AI.md](AI.md), the [rule index](guidance/rules/index.md), and the
[workspace model](docs/workspace-model.md). Check Git status and preserve
unrelated edits. For a defect, include a reproducible example; for a feature,
describe the intended behavior and ownership boundaries before expanding scope.

Use Python 3.9 or newer. The CLI and test suite use the standard library; no
package installation is required. Registered projects own their own dependencies
and setup.

## Making a change

- Inspect the implementation, callers, and tests before editing.
- Keep behavior changes focused and add regression coverage for observable
  behavior. Update directly related documentation.
- Keep workspace-owned and independent project repositories separate.
- Keep each workspace self-contained, describing only itself and its descendants.
  Never write upward workspace relationships into descendant documentation or
  make local guidance depend on context outside its root.
- Root `.agents/skills/` is canonical. Keep the corresponding copies under
  `templates/workspace/.agents/skills/` synchronized.
- Keep durable documentation with its owning project, use one canonical copy,
  and link rather than duplicate. Visible `workbench/` drafts may be tracked
  locally after review; do not commit them automatically. Keep restricted
  records outside documentation and Git, and promote approved knowledge to
  maintained docs.
- Template and skill changes affect generated output. Review a disposable
  generated workspace and ensure its own CLI can generate another workspace.
- Keep machine paths, credentials, provider assumptions, and framework-specific
  setup out of the generic starter.

## Verification

From this repository's root, run:

```sh
python3 -B tools/wsup.py validate
python3 -B tools/wsup.py guidance
python3 -B tools/wsup.py check --project wsup --show-output
git diff --check
```

For a focused test run, use the existing unittest runner:

```sh
python3 -B -m unittest discover -s tests -p 'test_services.py'
```

The configured suite covers standalone generation and update safety. Check
reports are local under ignored `.wsup/reports/`; keep development notes and
disposable artifacts in ignored `.work/`. Use temporary destinations outside
the source tree for manual generation and upgrade exercises; never overwrite
real user workspaces to test changes.

Report the interpreter, operating system, checks performed, and any skipped
coverage. Structural skill validation is not proof of client discovery or
invocation. See [AI guidance](docs/ai-guidance.md) for discovery and compatibility
limits. Keep session logs, implementation plans, and disposable investigation
material under ignored `.work/`; public documentation should describe the
product and its use.

## Preparing a contribution

Explain what changed, why, compatibility effects, and verification results.
Review the full diff, including new files. Keep local settings, reports, secrets,
and unrelated formatting out of the contribution.

Add user-visible changes to the Unreleased section of [CHANGELOG.md](CHANGELOG.md).
Ordinary contributions do not need a version bump or tag; maintainers group
changes into releases using the [release guide](docs/releases.md).

Contribute only material you have the right to submit under the repository's
[MIT license](LICENSE). This guide does not transfer copyright ownership.
