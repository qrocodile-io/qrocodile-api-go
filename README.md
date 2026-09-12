# qrocodile-api-go

Typed Go client for the [QRocodile QR Code API](https://qrocodile.io/en/qr-code-api/) — generate styled QR codes (SVG or PNG) from content plus a design config.

Requires Go 1.24 or newer (see the `go` directive in [go.mod](go.mod), the source of truth as this gets bumped).

## Install

```
go get github.com/qrocodile-io/qrocodile-api-go
```

## Get an API key

Every render needs a key. Register an email address, confirm the 6-digit code that arrives, and the key is shown exactly once.

```go
qr := qrocodile.NewClient("")
_, err := qr.RegisterKey(ctx, "you@example.com", "en")
// → a 6-digit code arrives by email; exchange it for the key, which is shown exactly once:
result, err := qr.ConfirmKey(ctx, "123456", qrocodile.ConfirmKeyIdentifier{Email: "you@example.com"})
apiKey := result.ApiKey
```

Registering an address that already has a key is how you replace a lost one: confirming rotates it, `result.Replaced` comes back `true`, and the old key stops working immediately.

You can also get a key from the [API page](https://qrocodile.io/en/qr-code-api/) without writing any code.

## Render

```go
qr := qrocodile.NewClient(os.Getenv("QR_API_KEY"))

// SVG (string) from simple content + a preset
svg, err := qr.RenderSVG(ctx, qrocodile.RenderInput{
	Content: "https://qrocodile.io",
	Design:  json.RawMessage(`{"preset":"ocean"}`),
})

// PNG ([]byte) with full design control. Payload formats are strings you build yourself —
// the API encodes `content` exactly as given.
png, err := qr.RenderPNG(ctx, qrocodile.RenderInput{Content: "WIFI:T:WPA;S:Cafe;P:latte123;;"})
```

There are two render methods, one per output: `RenderSVG` returns a `string`, `RenderPNG` returns a `[]byte`. Both take the same input — `Content` (the string to encode) plus optional `Design` (the full design config, as raw JSON), `Size`, and `FixContrast`. They call `POST /v1/qr`, which supports every option (the `GET /v1/qr` variant has no method yet).

`Design` is raw JSON — the object the QR Designer’s “Copy JSON” button produces, read from a file or built by hand. See the package doc comment on why the full design config isn’t exposed as a Go struct field-by-field. The quickest way to build one is the [QR Designer](https://qrocodile.io/en/) itself — style a code by hand, then use its “Copy JSON” button and paste the result straight in.

## Errors

```go
svg, err := qr.RenderSVG(ctx, qrocodile.RenderInput{Content: "https://qrocodile.io"})
var apiErr *qrocodile.APIError
if errors.As(err, &apiErr) {
	log.Println(apiErr.Status, apiErr.Code, apiErr.Message)
}
```

`Code` is a closed, spec-derived enum ([`qrocodile.ErrorCode`](qrocodile.go)) — branch on it, not on `Message`. A genuine network failure (offline, DNS, timeout) comes back as the underlying error instead, with no `*APIError` to unwrap.

## Other environments

```go
qrocodile.NewClient(apiKey, qrocodile.WithBaseURL("http://localhost:3002"))
```

## Full API documentation

Endpoint reference, design-config fields, error tables and rate limits: [qrocodile.io/en/qr-code-api/](https://qrocodile.io/en/qr-code-api/) and the OpenAPI UI at [api.qrocodile.io/docs](https://api.qrocodile.io/docs).

## License

MIT

## Contributing

Bug reports and questions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) if you are working on the client itself.
