# qrocodile-api-go

[![CI](https://github.com/qrocodile-io/qrocodile-api-go/actions/workflows/ci.yml/badge.svg)](https://github.com/qrocodile-io/qrocodile-api-go/actions/workflows/ci.yml)
[![Coverage](https://codecov.io/gh/qrocodile-io/qrocodile-api-go/graph/badge.svg)](https://codecov.io/gh/qrocodile-io/qrocodile-api-go)
[![Go Reference](https://pkg.go.dev/badge/github.com/qrocodile-io/qrocodile-api-go.svg)](https://pkg.go.dev/github.com/qrocodile-io/qrocodile-api-go)
[![Go version](https://img.shields.io/github/go-mod/go-version/qrocodile-io/qrocodile-api-go)](go.mod)
[![License](https://img.shields.io/github/license/qrocodile-io/qrocodile-api-go)](LICENSE)

Typed Go client for the [QRocodile QR Code API](https://qrocodile.io/en/qr-code-api/). It generates styled QR codes (SVG or PNG) from content plus a design config.

Requires Go 1.24 or newer (see the `go` directive in [go.mod](go.mod)).

## Install

```
go get github.com/qrocodile-io/qrocodile-api-go
```

## Get an API key

Every render needs a key. Register an email address, confirm the 6-digit code that arrives, and the key is shown exactly once.

```go
qr := qrocodile.NewClient("")
_, err := qr.RegisterKey(ctx, "you@example.com", "en")
// → a 6-digit code arrives by email; exchange it for the key:
result, err := qr.ConfirmKey(ctx, "123456", qrocodile.ConfirmKeyIdentifier{Email: "you@example.com"})
apiKey := result.ApiKey
```

To replace a lost or leaked key, register the same email address again. Confirming issues a new key, `result.Replaced` comes back `true`, and the old key stops working immediately.

You can also get a key from the [API page](https://qrocodile.io/en/qr-code-api/) without writing any code.

## Render

```go
qr := qrocodile.NewClient(os.Getenv("QR_API_KEY"))

// SVG (string) from simple content + a preset
svg, err := qr.RenderSVG(ctx, qrocodile.RenderInput{
	Content: "https://qrocodile.io",
	Design:  json.RawMessage(`{"preset":"ocean"}`),
})

// PNG ([]byte). Payload formats are strings you build yourself:
// the API encodes `content` exactly as given.
png, err := qr.RenderPNG(ctx, qrocodile.RenderInput{Content: "WIFI:T:WPA;S:Cafe;P:latte123;;"})
```

`RenderSVG` returns a `string`, `RenderPNG` returns a `[]byte`, and `Render` returns either as a `[]byte` together with a QR Designer link (see below). All three take the same input: `Content` (the string to encode) plus optional `Design` (the full design config, as raw JSON), `Size`, and `FixContrast`. They call `POST /v1/qr`, which supports every option. The `GET /v1/qr` variant has no method yet.

`Design` is raw JSON, not a Go struct. To build one, style a QR code in the [QR Designer](https://qrocodile.io/en/), choose its “Copy JSON” button, and paste the result in, either read from a file or inline. You can also write the JSON by hand. The package doc comment explains why the design config isn’t exposed field by field.

### Open in the QR Designer

To keep editing a code in the [QR Designer](https://qrocodile.io/en/), use `Render`. It returns the image together with a link that opens the same code (design and content) there:

```go
result, err := qr.Render(ctx, qrocodile.FormatPNG, qrocodile.RenderInput{Content: "https://qrocodile.io"})
// result.Image is the PNG; result.DesignerURL is the link.
```

The API only hands out a link that reproduces the code exactly. When it can’t (an uploaded logo, or content too long for a link), `DesignerURL` is empty and `DesignerURLUnavailable` names why, e.g. `["logo"]`.

## Errors

```go
svg, err := qr.RenderSVG(ctx, qrocodile.RenderInput{Content: "https://qrocodile.io"})
var apiErr *qrocodile.APIError
if errors.As(err, &apiErr) {
	log.Println(apiErr.Status, apiErr.Code, apiErr.Message)
}
```

Branch on `Code`, not on `Message`: the message is written for a human and may be reworded. `Code` is a [`qrocodile.ErrorCode`](qrocodile.go), and `qrocodile.ErrorCodes()` lists every code the API documents. A genuine network failure (offline, DNS, timeout) comes back as the underlying error instead, with no `*APIError` to unwrap.

## Full API documentation

Endpoint reference, design-config fields, error tables and rate limits: [qrocodile.io/en/qr-code-api/](https://qrocodile.io/en/qr-code-api/) and the OpenAPI UI at [api.qrocodile.io/docs](https://api.qrocodile.io/docs).

## License

MIT

## Contributing

Bug reports and questions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) if you are working on the client itself.
