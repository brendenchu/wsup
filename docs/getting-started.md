# Your first workspace

This macOS ARM64 walkthrough creates a new workspace, adds one workspace-owned
project, and runs an explicit check. It does not install dependencies, initialize
Git, change PATH, or need a Go toolchain.

## 1. Prepare the executable

This walkthrough uses the unreleased v0.3.0 layout. Until v0.3.0 assets are
published, build this branch with its pinned Go toolchain using the
[source-build instructions](../CONTRIBUTING.md#go-development). Download users
of v0.2.0 should use the
[v0.2.0 tutorial](https://github.com/brendenchu/wsup/blob/v0.2.0/docs/getting-started.md).
See [native binary use](native-install.md) for verification and recovery;
this tutorial uses an absolute executable path.

In the commands below, replace the `WSUP` value with the verified executable's
actual path. Choose a new `WORKSPACE` destination that does not already exist;
do not point it at an existing project or workspace. Run the shell snippets
in order in the same terminal:

```sh
WSUP="/absolute/path/to/current-build/wsup"
WORKSPACE="$HOME/wsup-first-workspace"
"$WSUP" --version
```

Confirm this is the intended trusted executable. Before the release version
is finalized, a development build may still print `0.2.0`; its source revision
must include the new layout. The published v0.2.0 binary cannot run this tutorial.

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
