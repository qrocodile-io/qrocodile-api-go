package qrocodile

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestIntegration_RenderSVG exercises a real render call against a live API instance. Skipped
// unless QR_API_KEY is set — set QR_API_INTEGRATION_REQUIRED=true to fail instead of skip (the
// mode CI runs in everywhere except fork PRs).
//
// QR_API_URL overrides the base URL (defaults to the production API) — this is how a local run
// points at a dev/staging instance, e.g. via a gitignored .env file that is never committed.
func TestIntegration_RenderSVG(t *testing.T) {
	apiKey := os.Getenv("QR_API_KEY")
	if apiKey == "" {
		if os.Getenv("QR_API_INTEGRATION_REQUIRED") == "true" {
			t.Fatal("QR_API_KEY is required (QR_API_INTEGRATION_REQUIRED=true)")
		}
		t.Skip("QR_API_KEY not set, skipping integration test")
	}

	var opts []ClientOption
	if baseURL := os.Getenv("QR_API_URL"); baseURL != "" {
		opts = append(opts, WithBaseURL(baseURL))
	}
	c := NewClient(apiKey, opts...)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	svg, err := c.RenderSVG(ctx, RenderInput{Content: "https://qrocodile.io"})
	if err != nil {
		t.Fatalf("RenderSVG: %v", err)
	}
	if !strings.Contains(svg, "<svg") {
		t.Errorf("response doesn't look like SVG: %s", firstN(svg, 200))
	}
}

func TestIntegration_RenderPNG(t *testing.T) {
	apiKey := os.Getenv("QR_API_KEY")
	if apiKey == "" {
		if os.Getenv("QR_API_INTEGRATION_REQUIRED") == "true" {
			t.Fatal("QR_API_KEY is required (QR_API_INTEGRATION_REQUIRED=true)")
		}
		t.Skip("QR_API_KEY not set, skipping integration test")
	}

	var opts []ClientOption
	if baseURL := os.Getenv("QR_API_URL"); baseURL != "" {
		opts = append(opts, WithBaseURL(baseURL))
	}
	c := NewClient(apiKey, opts...)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	png, err := c.RenderPNG(ctx, RenderInput{Content: "https://qrocodile.io"})
	if err != nil {
		t.Fatalf("RenderPNG: %v", err)
	}
	// PNG magic bytes: 0x89 'P' 'N' 'G'.
	if len(png) < 4 || string(png[1:4]) != "PNG" {
		t.Errorf("response doesn't look like PNG (%d bytes)", len(png))
	}
}

func TestIntegration_RenderSVG_invalidKeyIsAPIError(t *testing.T) {
	if os.Getenv("QR_API_KEY") == "" && os.Getenv("QR_API_INTEGRATION_REQUIRED") != "true" {
		t.Skip("QR_API_KEY not set, skipping integration test")
	}

	var opts []ClientOption
	if baseURL := os.Getenv("QR_API_URL"); baseURL != "" {
		opts = append(opts, WithBaseURL(baseURL))
	}
	c := NewClient("qk_live_definitely-not-a-real-key", opts...)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := c.RenderSVG(ctx, RenderInput{Content: "https://qrocodile.io"})
	if err == nil {
		t.Fatal("expected an error for an invalid key")
	}
	var apiErr *APIError
	if !asAPIError(err, &apiErr) {
		t.Fatalf("error is not *APIError: %v (%T)", err, err)
	}
	if apiErr.Status != 401 {
		t.Errorf("Status = %d, want 401", apiErr.Status)
	}
	if apiErr.Code != "UNAUTHORIZED" {
		t.Errorf("Code = %q, want UNAUTHORIZED", apiErr.Code)
	}
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
