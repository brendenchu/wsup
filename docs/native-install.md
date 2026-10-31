# Native binary use and recovery

WSUP runs as a separately supplied executable with embedded starter templates.
There is no installer, remote updater, or automatic PATH change. The published
v0.2.0 archive contains the compiled binary; download users do not need Go or a
source checkout. Only macOS ARM64 currently has runtime verification; other
target builds are not support claims.

The published v0.2.0 download uses the older layout. This branch documents
unreleased v0.3.0 behavior, including `wsup.json`, `wsup.lock`, and migration.
Use a binary built from this branch for those workflows; downloading v0.2.0
will not provide them. Use the
[v0.2.0 binary guide](https://github.com/brendenchu/wsup/blob/v0.2.0/docs/native-install.md)
when running the published binary.

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

## Optional PATH setup

Explicit-path invocation above is sufficient. To use the shorter `wsup` command,
keep the verified extracted package in a stable, owner-selected location, with
its license and notices. Substitute that package's actual absolute directory
below; do not use a temporary extraction directory you intend to delete:

```sh
export PATH="/absolute/path/to/wsup_0.2.0_darwin_arm64:$PATH"
command -v wsup
wsup --version
```

This changes only the current shell and places the chosen package before other
PATH entries. Confirm `command -v wsup` selects the intended executable and
`--version` prints `0.2.0`. No `sudo`, system-directory copy, or Go build is needed.

For persistence, manually add the reviewed `export PATH="..."` line, with your
real package directory, to your shell's startup file. macOS's default zsh login
shell reads `~/.zprofile`; Bash login shells use `~/.bash_profile`. Other shells
and terminal launch modes may use different files. Do not replace existing
startup-file contents or add duplicate lines. Open a new terminal and repeat
the two verification commands. WSUP does not edit these files.

To undo this setup, remove only the line you added and open a new terminal.
Before removing a package directory, confirm no PATH entry still depends on it.

## New and existing workspaces

```sh
/absolute/path/to/extracted/wsup init /new/workspace
/absolute/path/to/extracted/wsup init /new/project --template project
```

Destinations must not exist. The generated workspace contains managed files,
`wsup.json`, `wsup.lock`, and local reports under `workbench/.local/` when
checks run. It contains no
executable or copied catalogue. Retain access to a compatible external binary
after moving the workspace.

For a complete create/register/check walkthrough, follow the
[first-workspace tutorial](https://github.com/brendenchu/wsup/blob/v0.2.0/docs/getting-started.md).

The unreleased model uses `wsup.lock` version 1. Convert a workspace or project
created by wsup 0.2.0 with `wsup migrate`, previewing before `--apply`; see the
[migration guide](https://github.com/brendenchu/wsup/blob/main/docs/migrating.md).
Unsupported formats fail without writes.
Never edit schema numbers to bypass validation. Binary replacement alone does
not migrate files or state.

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
remove workspaces, `wsup.lock`, reports, projects, or local settings. WSUP has
no service, global credentials, telemetry, or uninstaller to run.
