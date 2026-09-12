# Contributing to qrocodile-api-go

Notes for working on the client itself. None of this reaches consumers — `internal/openapi` is enforced by the Go compiler, not just convention, so nothing here is importable from outside this module.

```bash
make build
make test
make check   # fmt, vet, lint, codegen-check, build, test — what CI runs
```

The hand-written surface in `qrocodile.go` is deliberately sealed: the generated OpenAPI types live in `internal/openapi` and are not re-exported wholesale — consumers depend only on `NewClient`, `RenderInput`, `APIError` and friends. Two flat, single-level enums (`PresetID`, `ErrorCode`) are re-exported directly, since aliasing them costs nothing; the full design config is not, for the reasons in the `qrocodile.go` package doc comment.

## Layout

| Path                                | What it is                                                        |
| ------------------------------------ | ------------------------------------------------------------------ |
| `qrocodile.go`                      | The entire public surface, plus the `Client` and its methods. Hand-written. |
| `errors.go`                         | `APIError`. Hand-written.                                         |
| `internal/openapi/types.gen.go`     | Generated. Never edit by hand — see Codegen below.                |
| `internal/openapi/openapi-snapshot.json` | The spec this was generated from, committed so a diff shows what changed. |
| `qrocodile_test.go`                 | Unit tests, `httptest` server, no network.                        |
| `integration_test.go`               | Hits the real API. Needs `QR_API_KEY`.                            |
| `cmd/codegen/`                      | The codegen tool — see Codegen below.                             |
| `cmd/changelog-section/`            | Extracts one version's `CHANGELOG.md` section for release notes — see Releasing below. |
| `examples/`                         | Runnable commands — see Examples below.                           |
| `tools/`                            | Its own Go module holding `golangci-lint` and `oapi-codegen` — see Tooling below. |
| `Makefile`                          | `make check` is what CI runs; see individual targets for the rest. |

## Tooling

`golangci-lint` and `oapi-codegen` live in `tools/go.mod`, a module of their own rather than a `tool` dependency of the main `go.mod`. Both pull in a large dependency tree with their own, newer Go-version requirements — bundled into the main module, that requirement becomes the floor every consumer of this client is forced onto just to `go get` it, for a linter and a codegen tool they never build. Splitting them out means `go.mod`'s `go` directive reflects only what the client's actual runtime dependency (`oapi-codegen/runtime`) needs.

`make tools` builds both into `tools/bin/` (gitignored) via `go install` from inside `tools/`, so the binaries land where the Makefile expects them without ever touching this module's dependency graph. `lint`, `codegen`, and `codegen-check` all depend on it and rebuild only when `tools/go.mod`/`tools/go.sum` change:

```bash
make tools    # builds tools/bin/golangci-lint and tools/bin/oapi-codegen
make lint
```

Bump a tool's version from inside `tools/`, the same way you would in any module: `cd tools && go get -tool <import path>@<version>`.

## Codegen

`internal/openapi/types.gen.go` is generated from the QR Render API's OpenAPI document, fetched from the live API at `https://api.qrocodile.io/docs/json`.

```bash
make codegen         # rewrite internal/openapi/types.gen.go
make codegen-check    # fail if it drifted from the API (runs in CI)
```

`oapi-codegen` is built by `make tools` (see Tooling above) into `tools/bin/oapi-codegen`; `cmd/codegen` shells out to that binary directly and fails with a clear message if it isn't there yet.

The API's OpenAPI document has no named `components/schemas` — every request/response shape, including the full design config, is inlined directly under its operation. Fed straight to `oapi-codegen`, that produces anonymous nested structs for `design` and a separately-named string-enum type per operation and status for `error.code` — neither usable as a public API surface. `cmd/codegen` preprocesses the document before generation to hoist the recurring shapes into named schemas instead: `design` becomes `QrDesignConfig`, the `error.code` enum (identical everywhere it appears, since it comes from one shared enum server-side) becomes `ErrorCode`, and the two success-response bodies become `RegisterKeyResult`/`ConfirmKeyResult`. If a future API change makes any of these hoists structurally invalid (e.g. `error.code` stops being identical across every response), the tool fails loudly with what it found rather than generating something subtly wrong — read the error before assuming it's a bug in the tool.

`-check` compares the freshly generated file against the committed one, so it can start failing without anything in this repo changing — which is why CI should run it nightly as well as on push. When it fails, the API has shipped something these types do not describe: regenerate and commit.

## Tests

```bash
make test
```

Two layers:

- **Unit** (`qrocodile_test.go`) — an `httptest.Server`, no network. Error mapping, the Bearer header, and the SVG-as-`string` vs PNG-as-`[]byte` split.
- **Integration** (`integration_test.go`) — hits a live API and self-skips unless `QR_API_KEY` is set, so `make test` stays offline by default:

  ```bash
  QR_API_KEY=qk_live_… make test
  # point at another environment:
  QR_API_KEY=qk_live_… QR_API_URL=http://localhost:3002 make test
  # fail instead of skip when the key is missing (what CI runs outside fork PRs):
  QR_API_KEY=qk_live_… make test-integration
  ```

  Keep these tests to a couple of renders — the account's budget is 60 renders per minute, shared with everything else using that key.

## Examples

Mirrors `qrocodile-api-node`'s `examples/` (`generate.ts`, `logo.ts`, `signup.ts`) — same flows, same env vars, one Go command per script:

```bash
go run ./examples/signup you@example.com
QR_API_KEY=qk_live_… go run ./examples/generate "https://qrocodile.io" png
QR_API_KEY=qk_live_… go run ./examples/logo "https://qrocodile.io" website svg
```

They build and vet like any other package (`make build`/`make vet` cover them), so they can't silently drift from the client's actual signature.

## Releasing

1. Move the `## Unreleased` section in `CHANGELOG.md` to a new `## X.Y.Z` heading.
2. Commit that, then tag and push:

   ```bash
   git tag v0.1.0
   git push --tags
   ```

Unlike npm, a Go module needs no publish step beyond the tag — the tag itself is the release, and `go get` resolves straight from it via the module proxy. The first time anyone resolves a tag, the proxy and checksum database cache it forever, so a bad release is fixed with a new tag, never a moved one.

Pushing a `v*` tag triggers `.github/workflows/release.yml`, which re-runs `make check` against that exact commit, then creates a GitHub release with notes copied from the `CHANGELOG.md` section for that version (via `cmd/changelog-section` — see its own doc comment). This step is optional from Go's point of view — `go get` needs no release page to exist — but it's where the changelog actually becomes visible to anyone browsing the repo. If the version has no section, or an empty one, the workflow fails on purpose before creating anything; the failure message explains how to safely move the tag, since nothing has been published yet at that point.

Pre-1.0, so a minor bump may carry breaking changes while the surface settles.
