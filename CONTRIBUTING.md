# Contributing to WSUP

WSUP is a local-first workspace CLI and starter, not a shared build system or an
AI-service integration. Keep changes generic, portable, and usable without this
checkout.

## Before changing code

Read [AGENTS.md](AGENTS.md) and the
[workspace model](templates/modules/workspace-base/files/docs/workspace-model.md). Check Git status and preserve
unrelated edits. For a defect, include a reproducible example; for a feature,
describe the intended behavior and ownership boundaries before expanding scope.

## Go development

The [published v0.3.0 release](https://github.com/wavykit/wsup/releases/tag/v0.3.0)
includes a compiled macOS ARM64 binary. Users of that archive need neither Go
nor this checkout; follow [native binary use](docs/native-install.md).
Compilation is required for source development and release packaging, not for
running the downloaded executable. The [first-workspace tutorial](docs/getting-started.md)
covers project registration and an explicit check without a development setup.

Use Go 1.27.3, pinned in `mise.toml`. Mise is optional if that toolchain is
already available. Development tests also require Git for ownership and reporting
assertions. The repository is [wavykit/wsup](https://github.com/wavykit/wsup),
and development source uses Go module `github.com/wavykit/wsup`. Keep internal
imports on that module path. This migration is unreleased; repository ownership
and a development build do not establish a published source-install channel.
See the [module transition](docs/native-install.md#source-module-transition).
From this root:

```sh
go test -race ./...
go vet ./...
go run ./cmd/wsup --help
go run ./cmd/wsup --version
```

Build outside the tracked tree with
`go build -o /path/to/local/output/wsup ./cmd/wsup`. Do not commit executables.
A compiled binary embeds its templates and needs no development toolchain for
workspace operations. Checks still require their configured tools. macOS ARM64
is the verified runtime; cross-compilation does not certify other targets.
Non-Unix check execution refuses launch until equivalent descendant supervision
is implemented.

The product version is `Version` in `internal/cli/cli.go`. Version 0.3.0 is the
published native release, dated 2026-10-31. The release page supplies the archive
and checksum manifest; a changelog date alone does not establish publication.
Ordinary changes do not imply a version bump.

## Sources and generated content

- `cmd/wsup/` is the executable entry point; `internal/` contains services and
  colocated regression tests.
- `templates/` is the complete starter authority: module manifests, static
  `files/`, and `text/template` `templates/` under `modules/<id>/`
  (`workspace-base`, `project-base`, one per collection, and presets) and the
  shared skill library under `skills/`, embedded directly
  by `templates/embed.go`. `internal/catalogue` validates every manifest. Embedding uses
  `all:` for hidden assets. There is no generated catalogue archive, runtime
  overlay, workspace-local override, or refresh command.
- `templates/` is the only copy of generated guidance, skills, and the
  workspace model. This repository is not itself a wsup workspace and has no
  root `.agents/` or registry. Tests verify generated
  output against `templates/`.
- `LICENSE` is the repository's license notice. Generated starters carry no
  license file; the license covers this repository only.
- Record CLI, starter, and skill changes together in root `CHANGELOG.md`.
  Starters are not independently released products and do not carry separate
  WSUP changelogs. Owners may create changelogs for their own projects.
- Keep WSUP repository-only material here, not in either starter: the product
  changelog, contributor guide, release procedures, and distribution tutorials.
  Starters carry only generated-owner documentation, agent instructions,
  configuration, and scaffold files.
- Never implement source changes by editing generated output.
- Keep generated workspaces/projects and test fixtures outside this checkout.
  GoReleaser uses ignored `dist/` and `.release-staging/`; run from an isolated
  checkout when artifacts must remain outside the working checkout.
  Development-only ignore rules guard common accidental root build outputs.
  Generated owners must track their own state for managed updates. Authored files under
  `templates/` are build inputs, not generated output.

Inspect implementation, callers, and tests before editing. Add regression
coverage for observable behavior and update directly related documentation.
Keep machine paths, credentials, provider assumptions, and framework setup out
of the generic starter. Each workspace knows only itself and its descendants;
do not introduce upward configuration or instruction inheritance.

Keep plans, drafts, and disposable evidence outside this repository. Keep
restricted records outside documentation and Git.

## Contracts to preserve

Checks execute literal argv without substitution or shell expansion. They use
the selected project directory, null stdin, bounded transient output, explicit
timeouts, and Unix process-group cancellation. Dependencies are not selected
implicitly. Ordinary failures do not stop later checks. Ctrl-C cancels remaining
work, writes a report, and returns 130. Descendants deliberately escaping a
process group are not guaranteed to be terminated; inherited-pipe cleanup is
bounded and cleanup failures are explicit.

Reports use schema 2 with Go/OS/architecture metadata, original command hashes,
input hashes, and sanitized evidence. They omit arguments, raw output, and
environment values. Command hashes use stable ASCII JSON strings separated by
comma-space; editor JSON also uses ASCII escapes, including surrogate pairs.
Missing Git is explicitly unavailable, never reported as a clean repository.
Exclusive JSON/Markdown creation uses mode 0600 on POSIX; this does not establish
equivalent Windows ACL privacy. Report pairs are not transactional.

Initialization requires a new destination and writes state last. It retains
partial output on I/O failure rather than deleting possible owner content.
Children always use embedded defaults, never parent selections or customizations.
Collection additions never adopt unmanaged files, even if bytes match.

Only schema-2 managed state is supported; other schemas fail without writes.
There is no in-place conversion or obsolete runtime/layout cleanup. Unsupported
workspaces need a new destination and explicit owner-managed transfer.
Content updates preserve registry ownership, custom baseline hashes, selected
collections, omitted homes, orphan bytes, reports, and unknown owner files.
All conflicts block apply. Observed inputs are rechecked; new files are exclusive,
existing replacements avoid in-place truncation, and state is written last.
I/O failures may leave partial content with old state. There is no automatic
backup/rollback, concurrent-writer isolation, or power-loss durability guarantee.
Recovery requires matched files and state, not edited schema numbers.

## Verification

From the repository root:

```sh
go test -race ./...
go vet ./...
git diff --check
```

For focused changes, select the affected packages:

```sh
go test -race ./internal/scaffolding
```

CI also generates a workspace and runs `validate` and `guidance` against it.

Native acceptance copies the source without installed state, builds offline from
cached Go dependencies using `-trimpath -buildvcs=false` and cgo disabled, checks
identical binary bytes across two output directories and the actual build
metadata, and runs the suite with only Go/Git on PATH and cgo
disabled. It excludes only that enclosing self-test to avoid recursion; the
full `go test -race ./...` run covers it. Standalone
generation, inspection, collection additions, updates, and child generation
also run with empty PATH.
Build and verify a local candidate with
the pinned GoReleaser and Go tools on macOS ARM64:

```sh
mise exec -- goreleaser release --snapshot --clean
mise exec -- go run ./scripts/release verify dist
```
See [local candidate archives](docs/releases.md#local-candidate-archive) for
prerequisites, notices, checksum verification, output, and separate release gates.

For starter changes, inspect a disposable workspace/project outside the checkout,
relocate output, and verify both safe changes and owner-byte preservation.
Never overwrite real workspaces to test changes. Report toolchain, OS/architecture,
checks performed, and skipped coverage. Structural guidance validation does not
prove client discovery or invocation; see [AI guidance](docs/ai-guidance.md).

## Preparing a contribution

Before editing, create or reuse a feature branch from the intended base; main
is protected. Open a pull request targeting `main`. The repository's solo-maintainer policy
requires a PR without requiring another person's approval. Resolve review
conversations and keep the branch up to date before merging. Reviews remain
welcome; the required approval count can increase when another maintainer joins.

The [CI workflow](.github/workflows/ci.yml) defines the stable `native-ci` job on
macOS ARM64 with the exact Go version from `go.mod`. It verifies module checksums,
runs race tests and vet, and checks the registry and guidance. All PRs run it,
including documentation changes. Configure it as a required GitHub Actions check
only after its first successful remote run; changing a required job name needs
a coordinated ruleset update. Local checks do not replace the remote gate.

CI uses read-only repository permissions and actions pinned to full commit SHAs.
Keep credentials out of the checkout and avoid privileged workflows that execute
contributor code. [Dependabot](.github/dependabot.yml) checks Go modules and Actions
weekly, groups minor/patch updates, and leaves major updates separate. Updates
use the same PR and CI process; they are not automatically approved or merged.
These GitHub configurations belong to this repository, not generated starters.

Explain behavior, compatibility effects, and actual verification. Review the
complete diff, including new files. Keep local settings, reports, secrets, and
unrelated formatting out of contributions. Add user-visible changes to
[CHANGELOG.md](CHANGELOG.md); ordinary contributions need no version bump or tag.
Use the [release guide](docs/releases.md) when maintainers select a release.
Commit, publication, installation, and deployment require explicit authorization.

Contribute only material you have the right to submit under the repository's
[MIT license](LICENSE). This guide does not transfer copyright ownership.
