// Command logo generates a QR code with a centered logo via the QRocodile QR Code API.
//
// Two kinds of logo, chosen automatically from the 2nd argument:
//
//   - a path to an image file (.png / .jpg / .svg) → embedded as a CUSTOM logo (base64,
//     validated + sanitized server-side; no URL fetching)
//
//   - anything else → treated as a BUILT-IN logo id (e.g. website, wifi, email)
//
//     QR_API_KEY=qk_live_… go run ./examples/logo "https://qrocodile.io" ./logo.png png
//     QR_API_KEY=… go run ./examples/logo "https://qrocodile.io" website svg
//     QR_API_URL=http://localhost:3002 QR_API_KEY=… go run ./examples/logo
//
// Uses RenderSVG/RenderPNG (POST /v1/qr), which take content plus a full design.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/qrocodile-io/qrocodile-api-go"
)

var mimeByExt = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".svg":  "image/svg+xml",
}

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
	logoArg := "website"
	if len(os.Args) > 2 {
		logoArg = os.Args[2]
	}
	format := "png"
	if len(os.Args) > 3 && os.Args[3] == "svg" {
		format = "svg"
	}

	logo, kind, err := buildLogo(logoArg)
	if err != nil {
		return err
	}

	design, err := json.Marshal(map[string]any{"preset": "classic", "logo": logo})
	if err != nil {
		return err
	}

	opts := []qrocodile.ClientOption{}
	if baseURL := os.Getenv("QR_API_URL"); baseURL != "" {
		opts = append(opts, qrocodile.WithBaseURL(baseURL))
	}
	qr := qrocodile.NewClient(apiKey, opts...)
	input := qrocodile.RenderInput{Content: content, Design: design}

	ctx := context.Background()
	file := "qr-logo." + format
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
	fmt.Fprintf(os.Stderr, "✓ Wrote %s  (content: %s, %s)\n", file, content, kind)
	return nil
}

// buildLogo returns the design.logo value — a custom (base64 image) or built-in (by id) logo —
// plus a human-readable description of which kind it picked.
func buildLogo(logoArg string) (map[string]any, string, error) {
	info, err := os.Stat(logoArg)
	if err != nil || info.IsDir() {
		// Not a file → a built-in logo id. Validated against the API's registry server-side
		// (a wrong id returns a clean 400).
		return map[string]any{
			"id":          logoArg,
			"color":       "#111111",
			"regionWidth": 0.25,
		}, fmt.Sprintf("built-in logo '%s'", logoArg), nil
	}

	mime, ok := mimeByExt[strings.ToLower(filepath.Ext(logoArg))]
	if !ok {
		return nil, "", fmt.Errorf("unsupported logo file type: %s (use .png, .jpg, or .svg)", logoArg)
	}
	data, err := os.ReadFile(logoArg)
	if err != nil {
		return nil, "", err
	}
	// A self-describing data URL — the client sends it as-is; the API validates the mime,
	// size, and dimensions, and sanitizes SVGs before embedding.
	dataURL := fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data))
	return map[string]any{
		"data":        dataURL,
		"regionWidth": 0.25,
	}, fmt.Sprintf("custom logo %s", logoArg), nil
}
