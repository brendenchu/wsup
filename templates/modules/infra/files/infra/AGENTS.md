# Infra

- Each project here is registered in `wsup.json`; its own README and
  AGENTS.md govern work inside it.
- Check the project's repository ownership before any Git action. An
  independent project is its own repository with a root `.gitignore` entry.
- Never commit credentials, state files, or environment secrets; reference
  their approved store.
- Planning against real environments is read-only work; applying, deploying,
  or destroying needs explicit authorisation for that environment.
