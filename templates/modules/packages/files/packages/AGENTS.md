# Packages

- Each project here is registered in `workspace.json`; its own README and
  AGENTS.md govern work inside it.
- Check the project's repository ownership before any Git action. An
  independent project is its own repository with a root `.gitignore` entry.
- Keep packages independent of application internals; consumers use a
  package's public interface and supported release, not its source paths.
- Treat a change to a public interface as a change for every consumer: inspect
  callers and state incompatibilities.
