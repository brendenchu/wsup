# wsup development

This repository is the wsup CLI: Go source, tests, and the starter content that
`wsup init` embeds and writes into generated workspaces and projects.

## Layout

- `cmd/wsup/`: entry point. `internal/`: CLI, catalogue, scaffolding,
  registry, checks, reporting, and guidance packages with colocated tests.
- `templates/`: the single source for everything generated, embedded by
  `templates/embed.go`: modules under `modules/<id>/` (a `module.json`
  manifest, `files/` copied as-is, and `templates/` rendered with Go
  `text/template` against `catalogue.RenderData`) and shared skills under
  `skills/`.
  Edit generated content here; there is no second copy elsewhere in the repo.
- `schema/`: the JSON Schema for `wsup.json`, embedded for `wsup schema` and
  served from the repository. Change it with `internal/registry/document.go`;
  a test keeps their fields in step.
- `docs/`: the product manual (install, getting started, releases, AI guidance).
- `.goreleaser.yaml`: native build/archive/cask configuration; `scripts/release/`
  prepares notices and verifies binary/archive acceptance.

This repository is not itself a wsup workspace: it has no registry, workspace
guidance, or skills of its own.

## Working rules

- Use Go from `mise.toml` (pinned in `go.mod`). Git is required by the tests.
- Before changing behavior, read the implementation, its tests, and the affected
  files under `templates/`. Read `templates/modules/workspace-base/files/docs/workspace-model.md`
  for registry, CLI, or update changes.
- Generated output must work without this checkout: no machine paths, client,
  provider, framework, or organization assumptions in `templates/`.
- Main is protected. Make changes on a feature branch and follow the PR/CI
  procedure in [CONTRIBUTING.md](CONTRIBUTING.md#preparing-a-contribution).
- Preserve unrelated work. Commit, push, tag, or publish only when asked.
- Record user-visible changes in `CHANGELOG.md` under Unreleased.

## Checks

```sh
go test -race ./...
go vet ./...
```

When changing init or update behavior, also generate a workspace and inspect it:

```sh
tmp=$(mktemp -d) && go run ./cmd/wsup init "$tmp/ws" && go run ./cmd/wsup --root "$tmp/ws" validate
```
