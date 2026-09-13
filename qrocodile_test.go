package qrocodile

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderSVG_success(t *testing.T) {
	var gotPath, gotAuth, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<svg>ok</svg>"))
	}))
	defer server.Close()

	c := NewClient("qk_live_test", WithBaseURL(server.URL))
	svg, err := c.RenderSVG(context.Background(), RenderInput{Content: "https://qrocodile.io"})
	if err != nil {
		t.Fatalf("RenderSVG: %v", err)
	}
	if svg != "<svg>ok</svg>" {
		t.Errorf("svg = %q", svg)
	}
	if gotPath != "/v1/qr" {
		t.Errorf("path = %q, want /v1/qr", gotPath)
	}
	if gotAuth != "Bearer qk_live_test" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if !strings.Contains(gotBody, `"format":"svg"`) {
		t.Errorf("body missing format=svg: %s", gotBody)
	}
	if !strings.Contains(gotBody, `"content":"https://qrocodile.io"`) {
		t.Errorf("body missing content: %s", gotBody)
	}
}

func TestRenderPNG_returnsBytes(t *testing.T) {
	pngBytes := []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0x01, 0x02}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()

	c := NewClient("qk_live_test", WithBaseURL(server.URL))
	png, err := c.RenderPNG(context.Background(), RenderInput{Content: "https://qrocodile.io"})
	if err != nil {
		t.Fatalf("RenderPNG: %v", err)
	}
	if string(png) != string(pngBytes) {
		t.Errorf("png = %v, want %v", png, pngBytes)
	}
}

func TestRenderSVG_designPassthrough(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<svg/>"))
	}))
	defer server.Close()

	c := NewClient("qk_live_test", WithBaseURL(server.URL))
	_, err := c.RenderSVG(context.Background(), RenderInput{
		Content: "https://qrocodile.io",
		Design:  json.RawMessage(`{"preset":"ocean"}`),
	})
	if err != nil {
		t.Fatalf("RenderSVG: %v", err)
	}
	design, ok := gotBody["design"].(map[string]any)
	if !ok {
		t.Fatalf("design field missing or wrong type in body: %v", gotBody)
	}
	if design["preset"] != "ocean" {
		t.Errorf("design.preset = %v, want ocean", design["preset"])
	}
}

func TestPost_apiErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":{"code":"UNPROCESSABLE","message":"content too long for this design"}}`))
	}))
	defer server.Close()

	c := NewClient("qk_live_test", WithBaseURL(server.URL))
	_, err := c.RenderSVG(context.Background(), RenderInput{Content: "https://qrocodile.io"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var apiErr *APIError
	if !asAPIError(err, &apiErr) {
		t.Fatalf("error is not *APIError: %v (%T)", err, err)
	}
	if apiErr.Status != 422 {
		t.Errorf("Status = %d, want 422", apiErr.Status)
	}
	if apiErr.Code != "UNPROCESSABLE" {
		t.Errorf("Code = %q, want UNPROCESSABLE", apiErr.Code)
	}
	if apiErr.Message != "content too long for this design" {
		t.Errorf("Message = %q", apiErr.Message)
	}
}

func TestPost_nonStandardErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>502 Bad Gateway</html>"))
	}))
	defer server.Close()

	c := NewClient("qk_live_test", WithBaseURL(server.URL))
	_, err := c.RenderSVG(context.Background(), RenderInput{Content: "https://qrocodile.io"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var apiErr *APIError
	if asAPIError(err, &apiErr) {
		t.Fatalf("expected a plain error for a non-standard body, got *APIError: %v", apiErr)
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("error should mention the status: %v", err)
	}
}

func TestRegisterKey_and_ConfirmKey(t *testing.T) {
	var registerBody, confirmBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/keys":
			_ = json.NewDecoder(r.Body).Decode(&registerBody)
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"message":"Check your email for the verification code to receive your API key."}`))
		case "/v1/keys/confirm":
			_ = json.NewDecoder(r.Body).Decode(&confirmBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"apiKey":"qk_live_new","replaced":false}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient("", WithBaseURL(server.URL))

	regResult, err := c.RegisterKey(context.Background(), "you@example.com", Lang("de"))
	if err != nil {
		t.Fatalf("RegisterKey: %v", err)
	}
	if regResult.Message == "" {
		t.Error("RegisterKeyResult.Message is empty")
	}
	if registerBody["email"] != "you@example.com" {
		t.Errorf("register body email = %v", registerBody["email"])
	}
	if registerBody["lang"] != "de" {
		t.Errorf("register body lang = %v, want de", registerBody["lang"])
	}

	confirmResult, err := c.ConfirmKey(context.Background(), "123456", ConfirmKeyIdentifier{Email: "you@example.com"})
	if err != nil {
		t.Fatalf("ConfirmKey: %v", err)
	}
	if confirmResult.ApiKey != "qk_live_new" {
		t.Errorf("ApiKey = %q", confirmResult.ApiKey)
	}
	if confirmResult.Replaced {
		t.Error("Replaced = true, want false")
	}
	if confirmBody["code"] != "123456" {
		t.Errorf("confirm body code = %v", confirmBody["code"])
	}
}

func TestConfirmKey_requiresEmailOrRID(t *testing.T) {
	c := NewClient("", WithBaseURL("http://unused.invalid"))
	_, err := c.ConfirmKey(context.Background(), "123456", ConfirmKeyIdentifier{})
	if err == nil {
		t.Fatal("expected an error when neither Email nor RID is set")
	}
}

// asAPIError is errors.As without importing errors into every test file twice — kept trivial
// on purpose.
func asAPIError(err error, target **APIError) bool {
	apiErr, ok := err.(*APIError)
	if !ok {
		return false
	}
	*target = apiErr
	return true
}

func TestAPIError_Error(t *testing.T) {
	err := &APIError{Status: 422, Code: "UNPROCESSABLE", Message: "content too long for this design"}
	got := err.Error()
	want := "qrocodile: content too long for this design (code UNPROCESSABLE, HTTP 422)"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// roundTripFunc adapts a function to http.RoundTripper, for injecting a fake transport via
// WithHTTPClient.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRenderSVG_networkFailure(t *testing.T) {
	networkErr := errors.New("connection refused")
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, networkErr
	})}

	c := NewClient("qk_live_test", WithBaseURL("http://unused.invalid"), WithHTTPClient(httpClient))
	_, err := c.RenderSVG(context.Background(), RenderInput{Content: "https://qrocodile.io"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var apiErr *APIError
	if asAPIError(err, &apiErr) {
		t.Fatalf("expected a plain error for a network failure, got *APIError: %v", apiErr)
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("error should wrap the network failure: %v", err)
	}
}

func TestRegisterKey_malformedResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	c := NewClient("", WithBaseURL(server.URL))
	_, err := c.RegisterKey(context.Background(), "you@example.com", "")
	if err == nil {
		t.Fatal("expected an error for a malformed response body")
	}
}

func TestConfirmKey_malformedResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	c := NewClient("", WithBaseURL(server.URL))
	_, err := c.ConfirmKey(context.Background(), "123456", ConfirmKeyIdentifier{Email: "you@example.com"})
	if err == nil {
		t.Fatal("expected an error for a malformed response body")
	}
}

func TestEnumValueLists_nonEmptyAndValid(t *testing.T) {
	if ids := PresetIDs(); len(ids) == 0 {
		t.Error("PresetIDs() is empty")
	} else {
		for _, id := range ids {
			if !id.Valid() {
				t.Errorf("PresetIDs() contains %q, which fails its own Valid()", id)
			}
		}
	}

	if ids := ModuleStyleIDs(); len(ids) == 0 {
		t.Error("ModuleStyleIDs() is empty")
	} else {
		for _, id := range ids {
			if !id.Valid() {
				t.Errorf("ModuleStyleIDs() contains %q, which fails its own Valid()", id)
			}
		}
	}

	if ids := FinderStyleIDs(); len(ids) == 0 {
		t.Error("FinderStyleIDs() is empty")
	} else {
		for _, id := range ids {
			if !id.Valid() {
				t.Errorf("FinderStyleIDs() contains %q, which fails its own Valid()", id)
			}
		}
	}

	if ids := LogoIDs(); len(ids) == 0 {
		t.Error("LogoIDs() is empty")
	} else {
		for _, id := range ids {
			if !id.Valid() {
				t.Errorf("LogoIDs() contains %q, which fails its own Valid()", id)
			}
		}
	}

	if codes := ErrorCodes(); len(codes) == 0 {
		t.Error("ErrorCodes() is empty")
	} else {
		for _, code := range codes {
			if !code.Valid() {
				t.Errorf("ErrorCodes() contains %q, which fails its own Valid()", code)
			}
		}
	}
}

func TestEnumValueLists_eachCallReturnsAnIndependentCopy(t *testing.T) {
	ids := PresetIDs()
	original := ids[0]
	ids[0] = "mutated"

	fresh := PresetIDs()
	if fresh[0] != original {
		t.Errorf("mutating one PresetIDs() call's result changed a later call's result: got %q, want %q — the two calls are sharing a backing array", fresh[0], original)
	}
}
