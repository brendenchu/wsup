# Your first workspace

This macOS ARM64 walkthrough creates a new workspace, adds one workspace-owned
project, and runs an explicit check. An optional second-project exercise uses
the source candidate to inspect registered work before selecting checks and
previewing managed updates. It does not install project dependencies,
initialize Git or change PATH. The downloaded binary needs no Go toolchain.

## 1. Prepare the executable

Follow [native binary use](native-install.md) to download both v0.3.0 assets,
verify the checksum, and extract the package. Optional PATH setup is in that
guide; this tutorial uses an absolute executable path instead.

Alternatively, follow the
[source candidate procedure](native-install.md#source-candidate-installation)
with Go 1.27.3 and use its explicit installed executable path below. That
commit-pinned development candidate is not a released source channel, even
though its CLI version still reports `0.3.0`.

In the commands below, replace the `WSUP` value with the verified executable's
actual path. Choose a new `WORKSPACE` destination that does not already exist;
do not point it at an existing project or workspace. Run the shell snippets
in order in the same terminal:

```sh
WSUP="/absolute/path/to/wsup_0.3.0_darwin_arm64/wsup"
WORKSPACE="$HOME/wsup-first-workspace"
"$WSUP" --version
```

Expected version: `0.3.0`. Confirm this is the intended trusted executable.
The published v0.2.0 binary cannot run this tutorial's new-layout commands.

## 2. Create and inspect a workspace

```sh
"$WSUP" init "$WORKSPACE"
"$WSUP" --root "$WORKSPACE" validate
"$WSUP" --root "$WORKSPACE" projects
"$WSUP" --root "$WORKSPACE" guidance
```

Initialization creates documentation, guidance, the `development` preset's
homes (`apps/`, `packages/`, `tools/`, `infra/`), an empty
`wsup.json` with no projects, and `wsup.lock`, which records what wsup installed.
`projects` initially prints `[]`; `guidance` should have no errors. These
commands do not run project checks. The package stays separate; the workspace
contains no executable.

`--root` goes before the command. Relative `init` destinations are resolved from
the terminal's current directory, not `--root`; this tutorial uses absolute
destinations to avoid that distinction.

## 3. Create a project

```sh
"$WSUP" init "$WORKSPACE/tools/readme-demo" --template project
```

This creates project documentation and guidance without choosing a programming
language or installing a runtime. The destination must not already exist.
Initialization does not add a registry entry automatically.

## 4. Register the project and a check

Open the new workspace's `wsup.json` in your editor and replace its empty
`projects` list with this one, keeping the other fields. Do not replace projects
you have already registered:

```json
{
  "projects": [
    {
      "id": "readme-demo",
      "path": "tools/readme-demo",
      "kind": "tool",
      "repository": "workspace",
      "purpose": "Learn explicit project checks",
      "owner": "Workspace owner",
      "dependencies": [],
      "tasks": {
        "readme": {"command": ["/usr/bin/grep", "-q", "^## Setup$", "README.md"], "check": true}
      }
    }
  ]
}
```

This example uses macOS's `/usr/bin/grep` to check that the project's starter
README has a `Setup` heading. It is an illustrative documentation check, not an
application test. WSUP executes the literal argument array in the project
directory without a shell; `^## Setup$` is a grep pattern, not shell expansion.
Replace this check with your project's real test/lint commands when appropriate,
and supply their tools separately.

`repository: "workspace"` declares that this project belongs to the workspace's
repository if you later create one; it does not initialize Git. For separately
owned repositories, follow [Git ownership](../templates/modules/workspace-base/files/docs/workspace-model.md#git-ownership)
before staging anything.

## 5. Validate and run the selected check

```sh
"$WSUP" --root "$WORKSPACE" validate
"$WSUP" --root "$WORKSPACE" projects
"$WSUP" --root "$WORKSPACE" guidance --project readme-demo
"$WSUP" --root "$WORKSPACE" check --project readme-demo --task readme --show-output
```

Validation should report one project, the listing should include `readme-demo`,
and the check should pass with exit code 0. `grep -q` has no normal output.
Checks run with your permissions; inspect registry commands before invoking
them. Nothing runs merely because a project is registered.

The check writes JSON and Markdown reports under `workbench/.local/wsup/reports/`. They include
statuses, hashes, and native version metadata, not command arguments or captured
output. Reports are ignored local evidence; review them before sharing. Git
metadata may be unavailable if Git is absent or no repository exists; it is
never reported as proof of a clean repository.

To observe a required-check failure, use your editor to temporarily rename
`## Setup` in the project's README and rerun the final command. It should return
1 and write a failure report. Restore that heading and rerun to get a pass.
A missing tool or invalid configuration is an error, not a successful check.

## 6. Inspect and maintain multiple projects (source candidate)

This section requires the
[verified source candidate](native-install.md#source-candidate-installation);
the downloaded v0.3.0 binary has no `projects ID` command. Use the candidate's
absolute path for `WSUP`. This is a runnable documentation-check example, not
evidence of application health or demonstrated productivity gains.

Create a second project:

```sh
"$WSUP" init "$WORKSPACE/tools/docs-demo" --template project
```

Append the following entry to the existing `projects` array in `wsup.json`,
keeping `readme-demo` and the other workspace fields:

```json
{
  "id": "docs-demo",
  "path": "tools/docs-demo",
  "kind": "tool",
  "repository": "workspace",
  "purpose": "Document the readme-demo workflow",
  "owner": "Workspace owner",
  "dependencies": ["readme-demo"],
  "tasks": {
    "readme": {"command": ["/usr/bin/grep", "-q", "^## Setup$", "README.md"], "check": true}
  }
}
```

Inspect before running:

```sh
"$WSUP" --root "$WORKSPACE" validate
"$WSUP" --root "$WORKSPACE" projects docs-demo
"$WSUP" --root "$WORKSPACE" projects readme-demo --json
"$WSUP" --root "$WORKSPACE" check --project docs-demo --task readme
"$WSUP" --root "$WORKSPACE" update
```

The readable view shows `docs-demo`'s registered owner, location, task arguments,
direct dependency and applicable instruction/skill paths. The JSON view for
`readme-demo` lists `docs-demo` as a dependent. These are configuration facts,
not executed instructions or proof that a task tool is available.

Only `docs-demo` is checked above. Dependencies are not automatic selections;
to check both, use:

```sh
"$WSUP" --root "$WORKSPACE" check --project readme-demo --project docs-demo --task readme
```

Review the report for the selected tasks and their results. Passing this example
means the two starter READMEs contain the checked heading, not that every
project or application behavior was verified.

`update` previews differences against the invoked binary's embedded starter
content. It neither downloads an executable nor upgrades project dependencies.
The source candidate may propose changes to content created by an older binary;
review the plan rather than expecting an empty result. Keep a matched backup
of files and `wsup.lock` before any `update --apply`; do not apply merely to
complete this tutorial. Owner-edited managed files require explicit resolution.

## Next steps

- Customize project documentation and register your real projects with explicit
  ownership and checks; see the [workspace model](../templates/modules/workspace-base/files/docs/workspace-model.md).
- Review [optional collections](../templates/modules/workspace-base/files/docs/workspace-model.md#default-layout-and-optional-collections)
  before adding organizational homes.
- Treat binary replacement and workspace-content updates separately. Read
  [replacement and recovery](native-install.md#replace-a-binary-and-recover),
  preserve a matched backup, and preview `update` before any `update --apply`.
- Keep this disposable workspace only if useful. Removing a test workspace is
  your own explicit decision; do not remove your downloaded package or other
  workspaces as part of cleanup.
