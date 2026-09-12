// Package qrocodile is a typed Go client for the QRocodile QR Code API — render styled QR
// codes (SVG or PNG) from content plus a design config, and issue API keys.
//
// A design config is passed as raw JSON via [RenderInput.Design], typically exported whole from
// the QR Designer's "Copy JSON" button, rather than built field-by-field as a Go struct — its
// shape is deeply nested and not a natural fit for one. [PresetID] and [ErrorCode] are exposed as
// enums so callers can validate against the same values the API accepts.
package qrocodile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/qrocodile-io/qrocodile-api-go/internal/openapi"
)

// DefaultBaseURL is the hosted QRocodile QR Code API.
const DefaultBaseURL = "https://api.qrocodile.io"

// PresetID is a built-in design preset, kept in sync with what the API accepts. See the QR
// Designer for what each preset actually looks like; this type only carries the IDs.
type PresetID = openapi.QrDesignConfigPreset

// ErrorCode is every failure code the API can return, as a closed enum kept in sync with the
// API. Branch on this, never on an error's message — see [APIError].
type ErrorCode = openapi.ErrorCode

// Lang selects the language of the verification email [Client.RegisterKey] sends. The zero
// value omits `lang` from the request, which the API defaults to "en".
type Lang = openapi.RegisterKeyJSONBodyLang

// QrDesignConfig is the full design config's type, exposed for JSON marshaling/unmarshaling
// convenience. Its own fields reference further types that are not exported (see the package
// doc) — construct a design by building or editing JSON, via [RenderInput.Design], rather than
// by populating this struct field-by-field in Go.
type QrDesignConfig = openapi.QrDesignConfig

// Client talks to the QRocodile QR Code API. Create one with [NewClient].
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// ClientOption configures a [Client] constructed by [NewClient].
type ClientOption func(*Client)

// WithBaseURL overrides the API base URL, e.g. to target a dev/staging instance.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) { c.baseURL = baseURL }
}

// WithHTTPClient overrides the *http.Client used for requests. Mainly for tests.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = httpClient }
}

// NewClient creates a client for the QRocodile QR Code API. apiKey may be empty for the
// key-registration flow ([Client.RegisterKey] / [Client.ConfirmKey]) — every other call needs
// one. Get a key via RegisterKey(email) → read the 6-digit code from the email → ConfirmKey(code,
// …).
func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:    DefaultBaseURL,
		apiKey:     apiKey,
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// RenderInput is the input to [Client.RenderSVG] and [Client.RenderPNG] — everything POST
// /v1/qr accepts except `format`, which each method picks by name instead.
type RenderInput struct {
	// Content is the value encoded into the QR code, verbatim — usually a URL, but any string
	// works, including payload formats such as `WIFI:T:WPA;S:Cafe;P:secret;;`, `mailto:…` or a
	// vCard. The API does not assemble those formats for you.
	Content string

	// Design is the full design — module and finder styles, colors, logo and halo — as raw
	// JSON, e.g. read from the file the QR Designer's "Copy JSON" button produces. Optional;
	// nil renders a plain, unstyled code. Use [PresetID] to validate a preset id if constructing
	// this by hand rather than passing through an exported file, e.g.
	// `json.RawMessage(`{"preset":"ocean"}`)`.
	Design json.RawMessage

	// Size is the image width and height in pixels. Defaults to 300 for SVG and 1,024 for PNG.
	Size *int

	// FixContrast nudges low-contrast color combinations apart so the QR code stays scannable.
	// Defaults to true server-side; set explicitly to false to get the colors exactly as given.
	FixContrast *bool
}

func (in RenderInput) requestBody(format string) ([]byte, error) {
	body := struct {
		Content     string          `json:"content"`
		Design      json.RawMessage `json:"design,omitempty"`
		Format      string          `json:"format"`
		Size        *int            `json:"size,omitempty"`
		FixContrast *bool           `json:"fixContrast,omitempty"`
	}{
		Content:     in.Content,
		Design:      in.Design,
		Format:      format,
		Size:        in.Size,
		FixContrast: in.FixContrast,
	}
	return json.Marshal(body)
}

// RenderSVG renders an SVG string (POST /v1/qr with format=svg).
func (c *Client) RenderSVG(ctx context.Context, in RenderInput) (string, error) {
	body, err := in.requestBody("svg")
	if err != nil {
		return "", fmt.Errorf("qrocodile: encoding request: %w", err)
	}
	respBody, err := c.post(ctx, "/v1/qr", body)
	if err != nil {
		return "", err
	}
	return string(respBody), nil
}

// RenderPNG renders PNG bytes (POST /v1/qr with format=png). Same input as [Client.RenderSVG].
func (c *Client) RenderPNG(ctx context.Context, in RenderInput) ([]byte, error) {
	body, err := in.requestBody("png")
	if err != nil {
		return nil, fmt.Errorf("qrocodile: encoding request: %w", err)
	}
	return c.post(ctx, "/v1/qr", body)
}

// RegisterKeyResult is what [Client.RegisterKey] resolves to.
type RegisterKeyResult = openapi.RegisterKeyResult

// RegisterKey starts API-key signup for an email address (POST /v1/keys). A 6-digit code is
// emailed; pass it to [Client.ConfirmKey] to reveal the key, shown exactly once. lang defaults
// to "en" when empty. Registering an address that already has a key is how you replace a lost
// one — confirming rotates it in place. The response is identical whether or not the address
// already has a key, so it cannot be used to probe which addresses are registered.
func (c *Client) RegisterKey(ctx context.Context, email string, lang Lang) (RegisterKeyResult, error) {
	reqBody := struct {
		Email string `json:"email"`
		Lang  Lang   `json:"lang,omitempty"`
	}{Email: email, Lang: lang}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return RegisterKeyResult{}, fmt.Errorf("qrocodile: encoding request: %w", err)
	}
	respBody, err := c.post(ctx, "/v1/keys", body)
	if err != nil {
		return RegisterKeyResult{}, err
	}
	var result RegisterKeyResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return RegisterKeyResult{}, fmt.Errorf("qrocodile: decoding response: %w", err)
	}
	return result, nil
}

// ConfirmKeyResult is what [Client.ConfirmKey] resolves to — the key, plus whether it replaced
// one.
type ConfirmKeyResult = openapi.ConfirmKeyResult

// ConfirmKeyIdentifier selects the pending signup [Client.ConfirmKey] is confirming: exactly one
// of Email or RID, matching the address it was requested for or the `rid` from the verification
// email's link.
type ConfirmKeyIdentifier struct {
	Email string
	RID   string
}

// ConfirmKey exchanges the 6-digit code from the verification email for the API key (POST
// /v1/keys/confirm). The key is returned exactly once — it is stored hashed and cannot be
// recovered.
func (c *Client) ConfirmKey(ctx context.Context, code string, id ConfirmKeyIdentifier) (ConfirmKeyResult, error) {
	if id.Email == "" && id.RID == "" {
		return ConfirmKeyResult{}, fmt.Errorf("qrocodile: ConfirmKeyIdentifier needs Email or RID")
	}
	reqBody := struct {
		Code  string `json:"code"`
		Email string `json:"email,omitempty"`
		Rid   string `json:"rid,omitempty"`
	}{Code: code, Email: id.Email, Rid: id.RID}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return ConfirmKeyResult{}, fmt.Errorf("qrocodile: encoding request: %w", err)
	}
	respBody, err := c.post(ctx, "/v1/keys/confirm", body)
	if err != nil {
		return ConfirmKeyResult{}, err
	}
	var result ConfirmKeyResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return ConfirmKeyResult{}, fmt.Errorf("qrocodile: decoding response: %w", err)
	}
	return result, nil
}

// post issues a POST to path and returns the raw response body on success. On an API error
// response (non-2xx with the standard {error:{code,message}} body) it returns an *[APIError].
// A genuine network failure (offline, DNS, timeout, ctx canceled) returns the underlying error
// instead — it never reached the API, so there is no status or code to attach.
func (c *Client) post(ctx context.Context, path string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("qrocodile: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qrocodile: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("qrocodile: reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseAPIError(resp.StatusCode, respBody)
	}
	return respBody, nil
}

func parseAPIError(status int, body []byte) error {
	var envelope openapi.ErrorResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		// The response wasn't the standard {error:{code,message}} shape — a proxy's own error
		// page, an empty body, whatever. Surface the raw body rather than hiding it behind a
		// decode error; there is no code to attach.
		return fmt.Errorf("qrocodile: HTTP %d: %s", status, string(body))
	}
	return &APIError{
		Status:  status,
		Code:    envelope.Error.Code,
		Message: envelope.Error.Message,
	}
}
