# docz-api

A Go API for docz repos

New to the repo? Start with [DEVELOPMENT.md](DEVELOPMENT.md) — toolchain
setup, running the service locally, Docker/compose, tests, and conventions.

## Quickstart

```sh
mise install                  # toolchain
just                          # task menu
just build                    # binary at build/bin/docz-api
just test                     # race detector
just run                      # build + run the binary
```

## Release

```sh
just release v0.1.0           # tags + pushes; CI runs goreleaser
```

Multi-arch archives land on the Forgejo (or GitHub) release page.
Version metadata (`version`, `commit`, `date`) is embedded via
`-ldflags` and surfaced in the binary's startup output.

## Container

```sh
docker build -t docz-api:dev \
  --build-arg VERSION=$(git describe --tags --always) \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) .
```

Image is distroless + nonroot; entrypoint is `docz-api`.

## Layout

```text
cmd/docz-api/    main package
internal/               library code (private to this module)
Dockerfile              multi-stage distroless build
.goreleaser.yml         release config
mise.toml               pinned toolchain
justfile                task runner
```

## Conventions

See `CLAUDE.md` for the full operating notes (Go-specific +
homelab universals).

## License

Apache-2.0
