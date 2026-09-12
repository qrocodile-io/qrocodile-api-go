// Command generate renders a QR code via the QRocodile QR Code API and saves it to a file.
//
//	QR_API_KEY=qk_live_… go run ./examples/generate "https://qrocodile.io" png
//	QR_API_URL=http://localhost:3002 QR_API_KEY=… go run ./examples/generate "hi" svg
//
// Args: [content] [format: svg|png]. Get a key with the signup example. RenderSVG/RenderPNG
// take the content string plus an optional design; the content is encoded exactly as given, so
// payload formats like "WIFI:T:WPA;S:…;;" are strings you build yourself.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/qrocodile-io/qrocodile-api-go"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Render failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	apiKey := os.Getenv("QR_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("set QR_API_KEY. Get one with: go run ./examples/signup <email>")
	}

	content := "https://qrocodile.io"
	if len(os.Args) > 1 {
		content = os.Args[1]
	}
	format := "png"
	if len(os.Args) > 2 && os.Args[2] == "svg" {
		format = "svg"
	}

	opts := []qrocodile.ClientOption{}
	if baseURL := os.Getenv("QR_API_URL"); baseURL != "" {
		opts = append(opts, qrocodile.WithBaseURL(baseURL))
	}
	qr := qrocodile.NewClient(apiKey, opts...)

	// design.preset is validated against the API's registry server-side (a wrong id returns a
	// clean 400) — swap "ocean" for any preset the API supports.
	input := qrocodile.RenderInput{
		Content: content,
		Design:  json.RawMessage(`{"preset":"ocean"}`),
	}

	ctx := context.Background()
	file := "qr." + format
	if format == "png" {
		png, err := qr.RenderPNG(ctx, input)
		if err != nil {
			return err
		}
		if err := os.WriteFile(file, png, 0o644); err != nil {
			return err
		}
	} else {
		svg, err := qr.RenderSVG(ctx, input)
		if err != nil {
			return err
		}
		if err := os.WriteFile(file, []byte(svg), 0o644); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "✓ Wrote %s  (content: %s, preset: ocean)\n", file, content)
	return nil
}
