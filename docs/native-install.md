# Native binary use and recovery

WSUP runs as a separately supplied executable with embedded starter templates.
There is no installer, remote updater, or automatic PATH change. The published
v0.2.0 archive contains the compiled binary; download users do not need Go or a
source checkout. Only macOS ARM64 currently has runtime verification; other
target builds are not support claims.

## Download

Obtain both files from the
[WSUP v0.2.0 release](https://github.com/brendenchu/wsup/releases/tag/v0.2.0):

- [wsup_0.2.0_darwin_arm64.tar.gz](https://github.com/brendenchu/wsup/releases/download/v0.2.0/wsup_0.2.0_darwin_arm64.tar.gz)
- [SHA256SUMS](https://github.com/brendenchu/wsup/releases/download/v0.2.0/SHA256SUMS)

Download the archive and manifest together. If replacing an earlier download,
keep the old package separately and use a fresh directory for the new pair.
Local `-local` candidate archives are separate from published release assets.

## Verify and extract

On macOS ARM64, from the directory containing both downloaded files:

```sh
shasum -a 256 -c SHA256SUMS
tar -tzf wsup_0.2.0_darwin_arm64.tar.gz
```

Stop if checksum verification fails. Review the archive listing:
it should contain one `wsup_0.2.0_darwin_arm64/` directory, a `wsup` executable,
`LICENSE`, this guide as `README.md`, `BUILD.txt`, and dependency notices under
`licenses/`. Extract
only a trusted archive into a new owner-selected directory. Checksums detect
different bytes, not authorship or safety when the manifest is also untrusted.
Published and local archives are unsigned and not notarized. Do not bypass
operating system security prompts for an untrusted executable.

For example, from the download directory, create a new extraction destination
and extract only after completing those checks:

```sh
mkdir wsup-0.2.0-extracted
tar -xzf wsup_0.2.0_darwin_arm64.tar.gz -C wsup-0.2.0-extracted
./wsup-0.2.0-extracted/wsup_0.2.0_darwin_arm64/wsup --version
```

Local candidates use `wsup_VERSION-local_darwin_arm64.tar.gz` and a matching
top-level directory instead. Substitute their exact names when following this
procedure; their version output does not establish a published release.

Keep the license and dependency notices when redistributing the executable.
`BUILD.txt` records source provenance, toolchain and binary build information.
Published packages identify the exact release tag and source commit; local
candidates also record working-tree status. Review it before redistributing.

## Run without installation

Use the extracted executable by absolute path:

```sh
/absolute/path/to/extracted/wsup --version
/absolute/path/to/extracted/wsup --help
/absolute/path/to/extracted/wsup --root /absolute/path/to/workspace validate
/absolute/path/to/extracted/wsup --root /absolute/path/to/workspace guidance
```

The executable's location does not select a workspace. Run from the workspace
root, or place global `--root` before the command. No Go toolchain, adjacent
checkout, or template directory is required. Checks still need their configured
executables and run with your permissions; WSUP never installs them.
Unix checks supervise process groups. Non-Unix check execution refuses launch.

An owner may separately copy the executable into a chosen writable location or
configure PATH, but neither extraction nor this procedure does that automatically.
Do not overwrite another executable or modify shell configuration implicitly.

## New and existing workspaces

```sh
/absolute/path/to/extracted/wsup init /new/workspace
/absolute/path/to/extracted/wsup init /new/project --template project
```

Destinations must not exist. The generated workspace contains managed files,
schema-2 state under `.wsup/`, and local reports when checks run. It contains no
executable or copied catalogue. Retain access to a compatible external binary
after moving the workspace.

Only schema-2 managed state is supported. Unsupported schemas fail without
writes; there is no in-place conversion. Never edit schema numbers to bypass
validation. Create a new destination and explicitly transfer reviewed owner
content instead, preserving the original workspace and reports.

## Replace a binary and recover

1. Keep the previous trusted executable in its own location. Extract and verify
   the next release or local candidate separately; do not replace a running
   executable in place.
2. Check its version and inspect a disposable generated workspace first. Run
   validation/guidance against the intended workspace before opting into checks.
3. Select the new executable by explicit path. Binary replacement does not
   update workspace content or rewrite state.
4. Before any content update, preserve a matched backup of files and state,
   including ignored owner material needed for recovery. Preview with `update`.
   Review every action/conflict before separately authorized `update --apply`.
   Updates do not download binaries and are not multi-file transactions.
5. If only the executable changed, select the previous trusted executable again.
   After a content/state change, binary selection alone is not rollback: restore
   matched files and state from the same backup and use a compatible executable.
   I/O failures can leave partial content; inspect before retrying.

To stop using an extracted executable, remove only its explicitly selected
binary/package files or any PATH entry the owner separately created. Do not
remove workspaces, `.wsup/` state/reports, projects, or local settings. WSUP has
no service, global credentials, telemetry, or uninstaller to run.
