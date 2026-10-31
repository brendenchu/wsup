# WSUP documentation

Maintain durable product and contributor documentation here:

- [Product quick start](../README.md): requirements, invocation, and everyday
  commands.
- [First-workspace tutorial](getting-started.md): create a workspace, register
  a project, and run a check with a binary built from this branch until v0.3.0 is published.
- [Workspace model](../templates/modules/workspace-base/files/docs/workspace-model.md): glossary, layout, module
  catalogue, presets, ownership, registry, `wsup.lock`, CLI configuration, and
  safe updates.
  Its [command reference](../templates/modules/workspace-base/files/docs/workspace-model.md#command-reference) covers options,
  output, root selection, and exit codes.
- [Migrating from wsup 0.2.0](migrating.md): preview, resolve conflicts,
  apply, and finish converting an existing workspace or project.
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

The [native template catalogue](../templates/) contains distributable starter
content, including the workspace model, which lives only there; this directory
documents the product itself.
