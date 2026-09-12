# Changelog

Notable changes to `qrocodile-api-go`. Versions follow [SemVer](https://semver.org/), with the usual pre-1.0 caveat: while the major is `0`, a **minor** bump may carry breaking changes.

## Unreleased

### Fixed

- Improve the documentation's clarity and accuracy throughout.

## 0.1.0

First public release.

### Added

- `NewClient(apiKey, opts...)` — a client for the QRocodile QR Code API, with `WithBaseURL` and `WithHTTPClient` options.
- `RenderSVG(ctx, input)` returns an SVG `string`; `RenderPNG(ctx, input)` returns PNG bytes as `[]byte`. Both take `RenderInput` (`Content` plus optional `Design`, `Size` and `FixContrast`) and call `POST /v1/qr`, which supports every option the API offers.
- `RegisterKey(ctx, email, lang)` and `ConfirmKey(ctx, code, ConfirmKeyIdentifier{Email or RID})` for programmatic API-key signup. `lang` is `Lang`, a closed enum generated from the OpenAPI document, not a bare string.
- `APIError`, carrying `Status` and a `Code` drawn from the API’s documented enum (`ErrorCode`), generated from the OpenAPI document so it can’t fall out of step with what the API actually returns. Network failures that never reached the API come back as the underlying error instead, untouched.
- `PresetID`, the API’s built-in preset IDs, also generated from the OpenAPI document.
- Runnable examples under `examples/`: signup, generate, and logo.

### Notes

- **Requires Go 1.24 or newer.** Development tooling (`golangci-lint`, `oapi-codegen`) lives in its own module under `tools/` specifically so its own, newer Go-version requirement never raises this floor.
- `Design` is raw JSON, not a typed struct — the design surface (presets, module and finder styles, palettes, logos, halos) is a deep tree of `oneOf` unions that Go’s type system has no clean way to express as a single struct, and the primary consumer forwards a design read whole from a file rather than constructing one field-by-field. See the `qrocodile.go` package doc comment for the full reasoning.
