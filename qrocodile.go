// Package qrocodile is a typed Go client for the QRocodile QR Code API. It renders styled QR
// codes (SVG or PNG) from content plus a design config, and issues API keys.
//
// A design config is passed as raw JSON via [RenderInput.Design], typically exported whole from
// the QR Designer's "Copy JSON" button, rather than built field-by-field as a Go struct, because its
// shape is deeply nested and not a natural fit for one. The flat, single-value fields that make
// sense as CLI-style flags or form inputs ([PresetID], [ModuleStyleID], [FinderStyleID],
// [LogoID], [ErrorCode]) are exposed as enums instead, each with a matching `<Type>s()`
// function (e.g. [PresetIDs]) returning every valid value, so callers can validate against or
// offer exactly what the API accepts. Those are functions, not package vars, so each call
// returns its own copy. A shared var would let one caller's in-place edit (a sort, or a filter
// that overwrites) corrupt the list for every other caller in the process.
package qrocodile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/qrocodile-io/qrocodile-api-go/internal/openapi"
)

// DefaultBaseURL is the hosted QRocodile QR Code API.
const DefaultBaseURL = "https://api.qrocodile.io"

// PresetID is a built-in design preset, kept in sync with what the API accepts. See the QR
// Designer for what each preset actually looks like; this type only carries the IDs.
type PresetID = openapi.QrDesignConfigPreset

// PresetIDs returns every valid PresetID, generated from the API's own OpenAPI document, so it
// can't fall out of step with what the API actually accepts. Use it to validate a preset before
// sending it, or to build a menu/completion list. Each call returns a fresh copy; safe to
// modify.
func PresetIDs() []PresetID { return append([]PresetID(nil), openapi.QrDesignConfigPresetValues...) }

// ModuleStyleID selects the shape the QR code's modules (the pattern) are drawn with.
type ModuleStyleID = openapi.QrDesignConfigModuleStyleId

// ModuleStyleIDs returns every valid ModuleStyleID, generated from the API's own OpenAPI
// document. Each call returns a fresh copy; safe to modify.
func ModuleStyleIDs() []ModuleStyleID {
	return append([]ModuleStyleID(nil), openapi.QrDesignConfigModuleStyleIdValues...)
}

// FinderStyleID selects the shape the three corner finders are drawn with.
type FinderStyleID = openapi.QrDesignConfigFinderStyleId

// FinderStyleIDs returns every valid FinderStyleID, generated from the API's own OpenAPI
// document. Each call returns a fresh copy; safe to modify.
func FinderStyleIDs() []FinderStyleID {
	return append([]FinderStyleID(nil), openapi.QrDesignConfigFinderStyleIdValues...)
}

// LogoID selects a built-in logo icon to place on the QR code. For your own artwork instead,
// build the `logo` field of [RenderInput.Design]'s JSON with a `data` key rather than `id`. See
// the API's design-config reference.
type LogoID = openapi.QrDesignConfigLogo0Id

// LogoIDs returns every valid LogoID, generated from the API's own OpenAPI document. Each call
// returns a fresh copy; safe to modify.
func LogoIDs() []LogoID { return append([]LogoID(nil), openapi.QrDesignConfigLogo0IdValues...) }

// ErrorCode is a failure code the API documents, generated from its OpenAPI document. Branch on
// this, never on an error's message; see [APIError].
type ErrorCode = openapi.ErrorCode

// ErrorCodes returns every valid ErrorCode, generated from the API's own OpenAPI document. Each
// call returns a fresh copy; safe to modify.
func ErrorCodes() []ErrorCode { return append([]ErrorCode(nil), openapi.ErrorCodeValues...) }

// Lang selects the language of the verification email [Client.RegisterKey] sends. The zero
// value omits `lang` from the request, which the API defaults to "en".
type Lang = openapi.RegisterKeyJSONBodyLang

// QrDesignConfig is the full design config's type, exposed for JSON marshaling/unmarshaling
// convenience. Its own fields reference further types that are not exported (see the package
// doc). Construct a design by building or editing JSON, via [RenderInput.Design], rather than
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
// key-registration flow ([Client.RegisterKey] / [Client.ConfirmKey]); every other call needs
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

// RenderInput is the input to [Client.Render], [Client.RenderSVG] and [Client.RenderPNG]:
// everything POST /v1/qr accepts except `format`, which Render takes as an argument and the
// other two imply by their name.
type RenderInput struct {
	// Content is the value encoded into the QR code, verbatim. It is usually a URL, but any string
	// works, including payload formats such as `WIFI:T:WPA;S:Cafe;P:secret;;`, `mailto:…` or a
	// vCard. The API does not assemble those formats for you.
	Content string

	// Design is the full design (module and finder styles, colors, logo and halo) as raw
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

// Format is the image format [Client.Render] asks for.
type Format string

const (
	// FormatSVG is SVG markup, UTF-8.
	FormatSVG Format = "svg"
	// FormatPNG is PNG bytes.
	FormatPNG Format = "png"
)

// RenderResult is what [Client.Render] returns: the image plus the link that opens it in the QR
// Designer.
type RenderResult struct {
	// Image is the rendered image: SVG markup for [FormatSVG], PNG bytes for [FormatPNG].
	Image []byte

	// DesignerURL opens the same code (design and content) in the QR Designer to keep editing
	// it, from the response's `Link: <…>; rel="edit-form"` header. Empty when the API cannot
	// build a link that reproduces the code exactly; DesignerURLUnavailable then says why.
	DesignerURL string

	// DesignerURLUnavailable names what the designer link cannot carry, from the
	// `X-Designer-Url-Unavailable` header. Each item is a design config path such as `logo`,
	// `content` for content the link cannot reproduce or fit, or `design` when the design alone
	// makes the link too long. Empty whenever DesignerURL is set.
	DesignerURLUnavailable []string
}

// Render renders a QR code in the given format (POST /v1/qr) and returns the image together with
// its designer link. [Client.RenderSVG] and [Client.RenderPNG] return only the image.
func (c *Client) Render(ctx context.Context, format Format, in RenderInput) (RenderResult, error) {
	if format != FormatSVG && format != FormatPNG {
		return RenderResult{}, fmt.Errorf("qrocodile: unknown format %q, want %q or %q", format, FormatSVG, FormatPNG)
	}
	body, err := in.requestBody(string(format))
	if err != nil {
		return RenderResult{}, fmt.Errorf("qrocodile: encoding request: %w", err)
	}
	respBody, header, err := c.do(ctx, "/v1/qr", body)
	if err != nil {
		return RenderResult{}, err
	}
	result := RenderResult{Image: respBody, DesignerURL: editFormLink(header.Values("Link"))}
	if result.DesignerURL == "" {
		result.DesignerURLUnavailable = splitList(header.Get("X-Designer-Url-Unavailable"))
	}
	return result, nil
}

// RenderSVG renders an SVG string (POST /v1/qr with format=svg).
func (c *Client) RenderSVG(ctx context.Context, in RenderInput) (string, error) {
	result, err := c.Render(ctx, FormatSVG, in)
	if err != nil {
		return "", err
	}
	return string(result.Image), nil
}

// RenderPNG renders PNG bytes (POST /v1/qr with format=png). Same input as [Client.RenderSVG].
func (c *Client) RenderPNG(ctx context.Context, in RenderInput) ([]byte, error) {
	result, err := c.Render(ctx, FormatPNG, in)
	if err != nil {
		return nil, err
	}
	return result.Image, nil
}

// linkEntry matches one `<url>; params` entry of a Link header (RFC 8288).
var linkEntry = regexp.MustCompile(`<([^>]*)>([^<]*)`)

// editFormLink returns the URL of the first `rel="edit-form"` link among Link header values, or
// "" without one.
func editFormLink(values []string) string {
	for _, value := range values {
		for _, m := range linkEntry.FindAllStringSubmatch(value, -1) {
			// The params run up to the next entry, so they may end in its separating comma.
			if hasRel(strings.TrimRight(strings.TrimSpace(m[2]), ","), "edit-form") {
				return m[1]
			}
		}
	}
	return ""
}

// hasRel reports whether a link's `; rel="…"` params name the relation, which may be one of
// several space-separated ones.
func hasRel(params, relation string) bool {
	for _, param := range strings.Split(params, ";") {
		name, value, ok := strings.Cut(param, "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(name), "rel") {
			continue
		}
		for _, rel := range strings.Fields(strings.Trim(strings.TrimSpace(value), `"`)) {
			if strings.EqualFold(rel, relation) {
				return true
			}
		}
	}
	return false
}

// splitList splits a comma-separated header value, dropping empty items.
func splitList(value string) []string {
	var items []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

// RegisterKeyResult is what [Client.RegisterKey] returns.
type RegisterKeyResult = openapi.RegisterKeyResult

// RegisterKey starts API-key signup for an email address (POST /v1/keys). A 6-digit code is
// emailed; pass it to [Client.ConfirmKey] to reveal the key, shown exactly once. lang defaults
// to "en" when empty. To replace a lost key, register the same address again: confirming issues
// a new key and the old one stops working. The response is identical whether or not the address
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

// ConfirmKeyResult is what [Client.ConfirmKey] returns: the key, plus whether it replaced one.
type ConfirmKeyResult = openapi.ConfirmKeyResult

// ConfirmKeyIdentifier selects the pending signup [Client.ConfirmKey] is confirming: exactly one
// of Email or RID, matching the address it was requested for or the `rid` from the verification
// email's link.
type ConfirmKeyIdentifier struct {
	Email string
	RID   string
}

// ConfirmKey exchanges the 6-digit code from the verification email for the API key (POST
// /v1/keys/confirm). The key is returned exactly once: the API stores only a hash, so it cannot be
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
// instead, because it never reached the API and has no status or code.
func (c *Client) post(ctx context.Context, path string, body []byte) ([]byte, error) {
	respBody, _, err := c.do(ctx, path, body)
	return respBody, err
}

// do is [Client.post] that also returns the response headers.
func (c *Client) do(ctx context.Context, path string, body []byte) ([]byte, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("qrocodile: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("qrocodile: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("qrocodile: reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, parseAPIError(resp.StatusCode, respBody)
	}
	return respBody, resp.Header, nil
}

func parseAPIError(status int, body []byte) error {
	var envelope openapi.ErrorResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		// The response wasn't the standard {error:{code,message}} shape (a proxy's own error
		// page, an empty body, whatever). Surface the raw body rather than hiding it behind a
		// decode error; there is no code to attach.
		return fmt.Errorf("qrocodile: HTTP %d: %s", status, string(body))
	}
	return &APIError{
		Status:  status,
		Code:    envelope.Error.Code,
		Message: envelope.Error.Message,
	}
}
