---
name: template-lifecycle
description: Create workspace or project starters, maintain template content, or preview and apply workspace updates. Use for init, update, and generated-workspace portability.
---

# Template lifecycle

1. Read the [workspace model](../../../docs/workspace-model.md) and affected
   starter guidance. Identify the operation: generate a new destination, change
   starter content, or update an existing workspace. Inspect CLI help and locate
   the actual template source; generated workspaces carry embedded templates and
   must work without the generator checkout.
2. For generation, use `python3 -B tools/wsup.py init DESTINATION` with
   `--template project` when appropriate. Choose a new destination and preserve
   existing paths. Inspect generated guidance, registry, and local links; project
   starters must remain usable outside their original workspace.
3. For starter maintenance, edit the owning source rather than a disposable
   generated result. In the generator repository, root `.ai/skills/` is canonical;
   synchronize its bundled template copies. Review both workspace and project
   surfaces if the change affects both, including embedded copies. Keep
   framework-specific setup with its project.
4. For an existing workspace update, inspect Git status and run
   `python3 -B tools/wsup.py update` first. Review proposed changes and conflicts.
   Apply with `update --apply` only when authorized and conflict-free. Preserve
   user-edited or missing files for manual resolution and retain the user-owned
   registry. The bundled template determines available updates; this is not a
   remote release download.
5. Verify changed starters in a disposable destination outside the source tree.
   Run generated workspace validation and guidance checks using its own CLI.
   When changing workspace generation, use that generated CLI to create another
   workspace and check its guidance too. For update changes, verify both safe
   updates and preservation of user edits with the existing tests.

Complete when the requested output or update is inspected and portable, existing
files remain preserved, and relevant checks pass. Report unresolved conflicts
and clean up only disposable paths created for verification.
