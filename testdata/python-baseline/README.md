# Python migration contracts

Reference: annotated `python-baseline`, commit
`3b709d522ddd1aab38c4442b20cfb22eb1787915` (product 0.1.0).
Captured and replayed with Python 3.9.6 on macOS ARM64. These are synthetic
fixtures, not private workspace captures or proof of other-platform support.

## Files

- `cases.json`: scenario inputs and ordered command steps. Each scenario runs
  in a fresh temporary directory. Setup edits precede the command snapshot.
- `expected.json`: captured exit status, stdout/stderr, file/directory writes,
  removed paths, preserved paths, and sanitized report evidence for every step.
- `test-map.json`: all 65 original Python test identifiers, behavior labels, and
  intended Go package/test targets. This frozen inventory retains its original
  **pending** statuses; current progress is recorded separately.
- `go-coverage.json`: reviewed Go assertion mappings for original contracts,
  distinguishing covered, partial, and unlisted/pending. Each mapping names
  existing Go tests and any remaining assertions. A Python test checks reference
  integrity; run Go tests separately to execute the assertions. Coverage here
  means the stated behavioral contract, not identical internal implementation.
- `diagnostic-cases.json`: malformed input, missing/non-file paths, environment
  errors, and argument failures for the approved native diagnostic policy.
  Unlike golden output, parser/runtime wording may differ; both runtimes must
  fail with exit 2, empty stdout, a handled stderr diagnostic, and no writes.
  Operational failures require the `Error: ` prefix; tracebacks are forbidden.
- `safety-contracts.json`: executable-reference test names and explicit setup,
  stimulus, and acceptance assertions for process cleanup, concurrent-state and
  write failures, migrations, omissions, collections, ownership, and reports.
  Additional Go requirements are identified separately from Python evidence.
- `generated-workspaces.json`: exact file sizes and SHA-256 digests, directory
  inventory, and source commit IDs for workspaces generated from real `v0.1.0`
  and the Python checkpoint. Regenerate the bytes from those immutable commits;
  the digest inventories are not replacement runtime bundles.

## Run

From the repository root:

```sh
python3 -B tests/migration_baseline.py
python3 -B tests/migration_baseline.py --verify-archives
python3 -B -m unittest discover -s tests -p test_migration_baseline.py
```

The first command replays against the working Python CLI. The second extracts
the exact recorded Git commits into temporary directories and verifies:

- Command expectations, the 65-test map, and both historical generation inventories.
- Actual upgrades of workspaces generated from both historical commits: read-only
  preview, all-write blocking on a customized README, explicit keep, preserved
  registry/project/local/report bytes, stable repeat preview, installed validation
  and guidance, and further workspace generation.
- All 65 original tests in the isolated checkpoint, requiring zero skips.
- The additional black-box safety probes against that isolated checkpoint.
- The diagnostic failure matrix against the isolated checkpoint.

It requires Git, the recorded objects locally, and a POSIX environment with
symlink support. Neither command changes expectations or real workspaces.
Original-test skips make archive verification fail rather than certify missing
coverage; platform-specific probes also report their skips explicitly.

The fixture runner also accepts an executable argv prefix:

```sh
python3 -B tests/migration_baseline.py --command /absolute/path/to/wsup
```

Full `--command` replay remains a future whole-port gate: Go scaffolding is not
ported. To compare inspection/editor commands while Python
drives setup and unported steps:

```sh
python3 -B tests/migration_baseline.py --compare-read-only /absolute/path/to/wsup
```

Compare native checks separately, letting Python drive other commands/setup:

```sh
python3 -B tests/migration_baseline.py --compare-checks /absolute/path/to/wsup
```

This mode changes only expected check-report schema versions from 1 to 2 and
validates actual Go/OS/architecture runtime metadata before runtime normalization.
All other command results, report fields, command/input hashes, literal argument
effects, byte counts, and non-report filesystem snapshots still compare exactly.
The Python expectations on disk remain unchanged. This mode requires `python3`
on PATH for the synthetic application checks, not for the Go CLI itself.

The subset comparison checks stdout, stderr, exits, no-write inspections, and exact
editor bytes/overwrite protection, followed by 20 diagnostic failure contracts.
It does not substitute Go for unported commands
or certify full parity. The synthetic check commands deliberately require
Python; they test running an application tool, not a dependency of the Go CLI.
The original Python tests remain the executable reference for internal failure
injection, timing/cancellation, migration, and ownership cases.
The supplemental probes can also be run directly:

```sh
python3 -B -m unittest discover -s tests -p test_migration_safety.py
python3 -B tests/test_migration_safety.py --source /path/to/extracted/python-baseline
```

## Comparison policy

- Parse JSON stdout before comparison; compare non-JSON stdout/stderr exactly.
- Normalize temporary absolute root/current-directory paths, generated report
  IDs, report timestamps/durations, and report runtime metadata only.
- Compare other report fields, including input/command hashes, exactly.
- Compare all non-report file bytes using SHA-256 plus byte length, recording
  unchanged paths and exact directory/symlink inventory. Reports compare parsed
  JSON evidence and timestamp-normalized Markdown rather than raw hashes.
- Report runtime normalization does not approve retaining `runtime.python` in Go:
  truthful runtime metadata and any schema version change remain a design gate.
- Do not normalize conflict/action lists, error content, exit codes, hashes, or
  lost files to make a port pass. One approved native difference is guidance
  error ordering: Go sorts errors lexically and preserves duplicates. The subset
  comparison sorts only Python's expected guidance errors; Go's actual order
  must match. Frozen Python expectations retain their original order.
- Native parser/runtime wording is separately approved. Only the explicit
  diagnostic matrix uses failure-shape assertions instead of byte equality;
  golden validation messages still compare exactly. This does not approve
  different accepted inputs or successful behavior.
- Help/version short-circuiting is a separate approved native parser difference:
  Cobra validates invalid options even alongside help/version, where Python may
  exit successfully before checking them. Direct Go tests assert exit 2 and no
  success output for these combinations; Python golden expectations are not
  changed to pretend the behavior is identical.

The current cases cover every command, version output, settings precedence and
invalid input, literal check arguments, failure continuation, report findings,
required skips, launch failure, output overflow, timeout, guidance errors,
editor overwrite protection, root resolution, workspace/project initialization,
collection preview/apply/collisions, update conflicts/keep, missing files,
manifest traversal, and symlink escapes/destinations.

The 71-step golden replay across thirteen scenarios is not a replacement for all 65
original tests. The Unicode editor scenario compares exact ASCII-escaped bytes,
including non-BMP characters, and preserves multiple guidance findings in order.
Guidance boundary scenarios cover colons in descriptions, carriage-return line
endings, shared-versus-legacy skill coexistence, and a symlink used as a scan root.
Cross-file finding order is an approved compatibility difference: Python scans
in filesystem traversal order, whereas Go sorts all error strings lexically.
The boundary scenario checks nested-versus-parent findings and duplicates;
Go tests also assert the ordering independently.
`safety-contracts.json` specifies how to port the safety cases that need
in-process injection or specialized probes rather than ordinary JSON fixtures.
Archive verification executes the original concurrency/I/O-failure, Git staging,
legacy migration, omission, and guidance tests. Supplemental black-box probes
verify real Ctrl-C with a descendant, retained pipes after immediate-parent exit,
and omitted-home persistence/independent child generation. Current Go mapping
progress is in `go-coverage.json`; unlisted contracts remain pending. Help text is not frozen: Go help formatting
may differ, but help must be useful and truthful.
Native installation/state changes and generated-runtime asset differences are
intentional-change candidates, not silently ignored comparisons.

The current live map records all 65 original contracts covered, none partial
or unlisted. Phase 5 acceptance now directly exercises collection content
upgrades, existing-directory preservation, omissions/new default markers, invalid
persisted state with no writes, all twelve legacy skill relocations, original
source retention on injected write failures, former-default orphan/catalogue
retirement, starter guidance locality, real Git ignore rules, and mixed Git
ownership/onboarding through native CLI commands. Mapping entries explicitly
identify approved external-binary/state-layout differences. The development/source
layout is verified by an isolated offline native-only source build and empty-PATH
generation/update checks. Complete native sources under `templates/` embed directly
without a generated ZIP or runtime overlay; `src/templates/` serves only the
separate Python reference. Repository guidance/checks now use native commands,
with an explicitly named Python reference check. This does not imply Python
reference tests have been retired.

Phase 1's Python behavior-freeze gate is verified on macOS ARM64/Python 3.9.6.
This is not Go parity, a native-runtime migration test, Windows descendant-cleanup
certification, or proof of transactional filesystem updates. Concurrent observed
file/tree/report races and late write failures have additional Go acceptance
requirements in the safety contracts; the existing Python tests verify only their
stated injection cases.

## Capture policy

Expectations are checked in and are never regenerated by normal tests:

```sh
python3 -B tests/migration_baseline.py --capture
```

Capture reads the pinned Python commit, never the working implementation, and
also generates the two historical inventories. Use only when deliberately
maintaining these contracts; review the resulting diff and rerun archive
verification. Do not move a tag or update the pinned commit to accommodate a
regression. Symlink and process-exit fixtures currently target the verified
macOS/POSIX environment; unsupported environments must report missing coverage
rather than claiming the same checks passed.
