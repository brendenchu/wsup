# WSUP documentation

Maintain durable product and contributor documentation here:

- [Product quick start](../README.md): requirements, invocation, and everyday
  commands.
- [First-workspace tutorial](getting-started.md): create a workspace, register
  a project, and run a check with the downloaded binary.
- [Workspace model](workspace-model.md): layout, ownership, registry, CLI
  configuration, optional collections, and safe updates.
  Its [command reference](workspace-model.md#command-reference) covers options,
  output, root selection, and exit codes.
- [AI guidance](ai-guidance.md): portable instructions, skills, and optional
  client discovery.
- [Releases](releases.md): current published release, versioning, and manual
  release procedure.
- [Native binary use](native-install.md): published downloads, archive
  verification, explicit-path invocation, replacement, uninstall, and recovery.
- [Contributing](../CONTRIBUTING.md): authoritative sources, starter maintenance,
  and local verification.

These documents describe the native CLI, not observed user adoption. macOS ARM64
is the verified runtime and has a compiled archive in the
[v0.2.0 release](https://github.com/brendenchu/wsup/releases/tag/v0.2.0).
Other targets are not support claims. Download users need no Go toolchain;
development verification uses the native Go suite and vet.

Keep drafts in the [development workbench](../workbench/). Session logs and
disposable evidence belong in ignored `workbench/.local/`, not public product docs.
The [native template catalogue](../templates/) contains distributable starter content;
this directory documents the product itself. The workspace model is also
bundled into the workspace starter and must remain synchronized.
