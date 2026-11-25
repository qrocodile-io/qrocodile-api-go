# Changelog

Notable changes to `qrocodile-api-go`. Versions follow [SemVer](https://semver.org/), with the usual pre-1.0 caveat: while the major is `0`, a **minor** bump may carry breaking changes.

## Unreleased

### Added

- Add the `koiPond`, `inkPond`, `beachTown`, `downtown`, and `mossWall` presets, and the `koi-pond`, `downtown`, and `moss` module styles.

## 0.3.0

### Added

- Add the `knit` module style and the `retroConsole` preset.
- Add the `bokeh`, `snowfall`, `gingerbread`, and `christmas-tree` module styles, the `bokeh` finder style, and the `cityLights`, `goldenHour`, `snowfall`, `winterNight`, `gingerbread`, and `christmasTree` presets.
- Add `Render(ctx, format, input)`, which returns the image together with a link that opens the code in the QR Designer, or the reason the API could not build one.

### Changed

- **Breaking:** rename the `christmas` preset to `christmasSweater`, as in the API. `PresetIDs()` no longer lists `christmas`, and a design that sets `"preset": "christmas"` needs to use `christmasSweater` instead.
- Follow the API’s tightened validation. It now refuses a `margin` below 1, gradient angles outside 0–360, style parameter values outside the range a style declares, and `#rrggbbaa` colors (only `#rgb` and `#rrggbb` remain).
- `finderUseModuleStyle` and `finderColorInherit` are documented as defaulting to `false`.

## 0.2.0

### Added

- `PresetIDs()`, `ModuleStyleIDs()`, `FinderStyleIDs()`, `LogoIDs()`, and `ErrorCodes()` return the full list of valid values for each enum type, generated from the API’s OpenAPI document so they match what the API accepts. Each call returns a fresh copy, so changing one caller’s result does not affect another’s. Before, only the types (`PresetID` and so on) were exposed, and there was no way to list their valid values.
- `ModuleStyleID`, `FinderStyleID`, and `LogoID`: three more single-value design-config enums, exposed the same way as `PresetID`, for the fields that are a flat choice rather than a nested object (see the package doc comment).

## 0.1.1

### Fixed

- Improve the documentation’s clarity and accuracy throughout.

## 0.1.0

First public release.

### Added

- `NewClient(apiKey, opts...)`, a client for the QRocodile QR Code API, with `WithBaseURL` and `WithHTTPClient` options.
- `RenderSVG(ctx, input)` returns an SVG `string`; `RenderPNG(ctx, input)` returns PNG bytes as `[]byte`. Both take `RenderInput` (`Content` plus optional `Design`, `Size` and `FixContrast`) and call `POST /v1/qr`, which supports every option the API offers.
- `RegisterKey(ctx, email, lang)` and `ConfirmKey(ctx, code, ConfirmKeyIdentifier{Email or RID})` for programmatic API-key signup. `lang` is `Lang`, an enum type generated from the OpenAPI document, not a bare string.
- `APIError`, carrying `Status` and a `Code` typed as `ErrorCode`, which lists the codes the API documents. It is generated from the OpenAPI document. Network failures that never reached the API come back as the underlying error instead, untouched.
- `PresetID`, the API’s built-in preset IDs, also generated from the OpenAPI document.
- Runnable examples under `examples/`: signup, generate, and logo.

### Notes

- **Requires Go 1.24 or newer.** Development tooling (`golangci-lint`, `oapi-codegen`) lives in its own module under `tools/` specifically so its own, newer Go-version requirement never raises this floor.
- `Design` is raw JSON, not a typed struct. The design surface (presets, module and finder styles, palettes, logos, halos) is a deep tree of `oneOf` unions that Go’s type system has no clean way to express as a single struct, and callers typically pass on a design read whole from a file instead of building one field by field. The `qrocodile.go` package doc comment has the full reasoning.
