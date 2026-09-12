// Command codegen regenerates internal/openapi/types.gen.go from the QRocodile API's OpenAPI
// document.
//
// The document has no named components/schemas — every request/response shape, including the
// full design config, is inlined directly under its operation. Fed straight to oapi-codegen,
// that produces anonymous nested structs for `design` and a separate,
// differently-named string-enum type for `error.code` per operation and status — unusable as a
// public API surface. This tool hoists both into named schemas first:
//
//   - `design` (from POST /v1/qr's request body) becomes `#/components/schemas/QrDesignConfig`.
//   - The `error.code` enum, repeated identically across every error response in the document
//     (they all come from one shared Zod enum server-side), becomes `#/components/schemas/ErrorCode`.
//
// Run with `go run ./cmd/codegen`. Add `-check` to fail if the committed generated file is
// out of date instead of overwriting it — the mode a CI drift check runs.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"time"
)

// defaultSpecURL is the production document — the committed generated code describes the public
// contract, not whatever a developer's local/dev instance happens to be running.
const defaultSpecURL = "https://api.qrocodile.io/docs/json"

func main() {
	check := flag.Bool("check", false, "fail if the generated file differs, instead of writing it")
	specURL := flag.String("spec-url", defaultSpecURL, "OpenAPI document URL to generate from")
	flag.Parse()

	if err := run(*check, *specURL); err != nil {
		fmt.Fprintln(os.Stderr, "codegen:", err)
		os.Exit(1)
	}
}

func run(check bool, specURL string) error {
	repoRoot, err := repoRoot()
	if err != nil {
		return err
	}

	raw, err := fetchSpec(specURL)
	if err != nil {
		return fmt.Errorf("fetching spec: %w", err)
	}

	snapshotPath := filepath.Join(repoRoot, "internal", "openapi", "openapi-snapshot.json")
	if err := writeFilePretty(snapshotPath, raw); err != nil {
		return fmt.Errorf("writing spec snapshot: %w", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parsing spec: %w", err)
	}

	if err := hoistDesignConfig(doc); err != nil {
		return fmt.Errorf("hoisting QrDesignConfig: %w", err)
	}
	if err := hoistErrorCode(doc); err != nil {
		return fmt.Errorf("hoisting ErrorCode: %w", err)
	}
	if err := hoistNamedResponse(doc, "RegisterKeyResult", "/v1/keys", "post", "202"); err != nil {
		return fmt.Errorf("hoisting RegisterKeyResult: %w", err)
	}
	if err := hoistNamedResponse(doc, "ConfirmKeyResult", "/v1/keys/confirm", "post", "200"); err != nil {
		return fmt.Errorf("hoisting ConfirmKeyResult: %w", err)
	}
	if err := hoistErrorResponse(doc); err != nil {
		return fmt.Errorf("hoisting ErrorResponse: %w", err)
	}

	preprocessed, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("re-encoding preprocessed spec: %w", err)
	}

	tmp, err := os.CreateTemp("", "qrocodile-openapi-*.json")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(preprocessed); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	configPath := filepath.Join(repoRoot, "internal", "openapi", "codegen.yaml")
	// oapi-codegen's version is pinned in tools/go.mod, a separate module so its own Go-version
	// requirement never raises the floor this module's own consumers need — see CONTRIBUTING.md.
	// `make tools` builds this binary; it does not build itself on demand.
	oapiCodegenPath := filepath.Join(repoRoot, "tools", "bin", "oapi-codegen")
	if _, err := os.Stat(oapiCodegenPath); err != nil {
		return fmt.Errorf("oapi-codegen binary not found at %s — run `make tools` first", oapiCodegenPath)
	}
	cmd := exec.Command(oapiCodegenPath, "-config", configPath, tmp.Name())
	cmd.Stderr = os.Stderr
	generated, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("running oapi-codegen: %w", err)
	}

	outPath := filepath.Join(repoRoot, "internal", "openapi", "types.gen.go")

	if check {
		existing, err := os.ReadFile(outPath)
		if err != nil {
			return fmt.Errorf("reading committed %s: %w", outPath, err)
		}
		if !bytes.Equal(existing, generated) {
			return fmt.Errorf("%s is out of date with %s — run `go run ./cmd/codegen` and commit the result", outPath, specURL)
		}
		fmt.Println("codegen: up to date")
		return nil
	}

	if err := os.WriteFile(outPath, generated, 0o644); err != nil {
		return err
	}
	fmt.Println("codegen: wrote", outPath)
	return nil
}

func fetchSpec(url string) ([]byte, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// hoistDesignConfig moves POST /v1/qr's inline `design` schema to
// #/components/schemas/QrDesignConfig and replaces it with a $ref, so oapi-codegen emits a named
// Go type instead of an anonymous nested struct.
func hoistDesignConfig(doc map[string]any) error {
	design, err := digGet(doc, "paths", "/v1/qr", "post", "requestBody", "content", "application/json", "schema", "properties", "design")
	if err != nil {
		return err
	}
	designSchema, ok := design.(map[string]any)
	if !ok {
		return fmt.Errorf("design schema is not an object: %T", design)
	}

	if err := setComponentSchema(doc, "QrDesignConfig", designSchema); err != nil {
		return err
	}

	props, err := digGet(doc, "paths", "/v1/qr", "post", "requestBody", "content", "application/json", "schema", "properties")
	if err != nil {
		return err
	}
	props.(map[string]any)["design"] = map[string]any{"$ref": "#/components/schemas/QrDesignConfig"}
	return nil
}

// hoistErrorCode finds every `error.code` enum schema in the document, checks they're all
// identical (they come from one shared Zod enum server-side), hoists a hand-authored
// #/components/schemas/ErrorCode carrying that enum, and replaces every occurrence with a $ref.
//
// The schema is authored here rather than copied from whichever occurrence the walk visits
// first: `walk` traverses a map[string]any tree, and Go's map iteration order is randomized
// per run, so copying a found schema's own `description` (varying only cosmetically, if at
// all, between occurrences) would make the generated file's doc comment — and therefore the
// file itself — nondeterministic between runs of this tool. Copying only the verified-identical
// `enum` list, never any occurrence's own fields, keeps the output the same every time.
func hoistErrorCode(doc map[string]any) error {
	var enums [][]any

	walk(doc, func(node map[string]any) {
		errorProp, ok := node["error"].(map[string]any)
		if !ok {
			return
		}
		errorProps, ok := errorProp["properties"].(map[string]any)
		if !ok {
			return
		}
		codeSchema, ok := errorProps["code"].(map[string]any)
		if !ok {
			return
		}
		enum, ok := codeSchema["enum"].([]any)
		if !ok {
			return
		}
		enums = append(enums, enum)
	})

	if len(enums) == 0 {
		return fmt.Errorf("no error.code enum schemas found — has the API's error shape changed?")
	}
	for _, enum := range enums[1:] {
		if !reflect.DeepEqual(enum, enums[0]) {
			return fmt.Errorf("error.code enums are not identical across the document (%v vs %v) — the shared-enum assumption no longer holds, hand-review before hoisting", enums[0], enum)
		}
	}

	if err := setComponentSchema(doc, "ErrorCode", map[string]any{
		"type":        "string",
		"description": "Machine-readable failure code. Stable across releases, so this is the field to branch on. `message` is not stable — it is written for a human reading a log and may be reworded at any time.",
		"enum":        enums[0],
	}); err != nil {
		return err
	}

	// Second pass to rewrite every occurrence: doing this after validation (rather than
	// collecting mutable references during the same walk) keeps the walk callback pure —
	// find-only — matching hoistErrorResponse's shape below and avoiding a codeSchemaRefs
	// slice that exists only to be mutated once, later.
	walk(doc, func(node map[string]any) {
		errorProp, ok := node["error"].(map[string]any)
		if !ok {
			return
		}
		errorProps, ok := errorProp["properties"].(map[string]any)
		if !ok {
			return
		}
		if _, ok := errorProps["code"].(map[string]any); !ok {
			return
		}
		errorProps["code"] = map[string]any{"$ref": "#/components/schemas/ErrorCode"}
	})
	return nil
}

// hoistNamedResponse hoists the JSON response schema at paths[path][method].responses[status]
// to #/components/schemas/<name> and replaces it with a $ref — for the handful of success
// bodies (RegisterKeyResult, ConfirmKeyResult) that, like `design`, are inlined rather than
// $ref'd, and only exist once each so there's nothing to deduplicate.
func hoistNamedResponse(doc map[string]any, name, path, method, status string) error {
	schema, err := digGet(doc, "paths", path, method, "responses", status, "content", "application/json", "schema")
	if err != nil {
		return err
	}
	schemaMap, ok := schema.(map[string]any)
	if !ok {
		return fmt.Errorf("response schema is not an object: %T", schema)
	}
	if err := setComponentSchema(doc, name, schemaMap); err != nil {
		return err
	}
	responses, err := digGet(doc, "paths", path, method, "responses", status)
	if err != nil {
		return err
	}
	content := responses.(map[string]any)["content"].(map[string]any)
	content["application/json"] = map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/" + name}}
	return nil
}

// hoistErrorResponse finds every `{ error: { code, message } }` response schema — by this point
// `code` is already a $ref to ErrorCode (hoistErrorCode ran first) — hoists a hand-authored
// #/components/schemas/ErrorResponse, and replaces every one of them with a $ref.
//
// The schema is authored here rather than copied from whichever response the traversal visits
// first, for two reasons: map iteration order is randomized per run (see hoistErrorCode), and
// — independent of that — each response's own `description`/`example` is specific to that
// endpoint and status ("the design was malformed", say), which would be actively misleading
// attached to a type meant to represent every error response generically.
func hoistErrorResponse(doc map[string]any) error {
	paths, err := digGet(doc, "paths")
	if err != nil {
		return err
	}

	found := false
	for _, pathItem := range paths.(map[string]any) {
		methods, ok := pathItem.(map[string]any)
		if !ok {
			continue
		}
		for _, op := range methods {
			opMap, ok := op.(map[string]any)
			if !ok {
				continue
			}
			responses, ok := opMap["responses"].(map[string]any)
			if !ok {
				continue
			}
			for _, resp := range responses {
				respMap, ok := resp.(map[string]any)
				if !ok {
					continue
				}
				content, ok := respMap["content"].(map[string]any)
				if !ok {
					continue
				}
				jsonContent, ok := content["application/json"].(map[string]any)
				if !ok {
					continue
				}
				schema, ok := jsonContent["schema"].(map[string]any)
				if !ok || !looksLikeErrorEnvelope(schema) {
					continue
				}
				found = true
				jsonContent["schema"] = map[string]any{"$ref": "#/components/schemas/ErrorResponse"}
			}
		}
	}

	if !found {
		return fmt.Errorf("no {error:{code,message}} response schema found — has the API's error shape changed?")
	}
	return setComponentSchema(doc, "ErrorResponse", map[string]any{
		"type":     "object",
		"required": []any{"error"},
		"properties": map[string]any{
			"error": map[string]any{
				"type":     "object",
				"required": []any{"code", "message"},
				"properties": map[string]any{
					"code": map[string]any{"$ref": "#/components/schemas/ErrorCode"},
					"message": map[string]any{
						"type":        "string",
						"description": "What went wrong, in English, for a human to read. Do not match on it — see `code`.",
					},
				},
			},
		},
	})
}

// looksLikeErrorEnvelope reports whether schema is shaped `{ type: object, properties: { error:
// { properties: { code: {$ref: .../ErrorCode}, message: {type: string} } } } }`. Deliberately
// ignores `description`/`example`, which differ legitimately per endpoint even though the
// structure — and therefore the Go type oapi-codegen would generate — is identical.
func looksLikeErrorEnvelope(schema map[string]any) bool {
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return false
	}
	errorSchema, ok := props["error"].(map[string]any)
	if !ok {
		return false
	}
	errorProps, ok := errorSchema["properties"].(map[string]any)
	if !ok {
		return false
	}
	code, ok := errorProps["code"].(map[string]any)
	if !ok || code["$ref"] != "#/components/schemas/ErrorCode" {
		return false
	}
	message, ok := errorProps["message"].(map[string]any)
	if !ok || message["type"] != "string" {
		return false
	}
	return true
}

func setComponentSchema(doc map[string]any, name string, schema map[string]any) error {
	components, ok := doc["components"].(map[string]any)
	if !ok {
		components = map[string]any{}
		doc["components"] = components
	}
	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		schemas = map[string]any{}
		components["schemas"] = schemas
	}
	schemas[name] = schema
	return nil
}

// digGet walks a chain of map keys, erroring out (rather than panicking) the moment one is
// missing or not a map — this runs against a live document fetched over the network, and a
// future API change reshaping any of these paths should fail loudly here, not deep inside
// oapi-codegen with a confusing error.
func digGet(doc map[string]any, path ...string) (any, error) {
	var cur any = doc
	for i, key := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("path %v: %q is not an object (at segment %d)", path, key, i)
		}
		next, ok := m[key]
		if !ok {
			return nil, fmt.Errorf("path %v: missing %q (at segment %d) — has the API's schema shape changed?", path, key, i)
		}
		cur = next
	}
	return cur, nil
}

// walk visits every map[string]any node in the document, depth-first. Order isn't significant —
// hoistErrorCode only needs "find every matching node," not a particular traversal order.
func walk(node any, visit func(map[string]any)) {
	switch v := node.(type) {
	case map[string]any:
		visit(v)
		for _, child := range v {
			walk(child, visit)
		}
	case []any:
		for _, child := range v {
			walk(child, visit)
		}
	}
}

func writeFilePretty(path string, raw []byte) error {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, pretty, 0o644)
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	// Invoked via `go run ./cmd/codegen` from the repo root, so the working directory already
	// is the repo root — this just guards against an accidental invocation from elsewhere.
	if _, err := os.Stat(filepath.Join(wd, "go.mod")); err != nil {
		return "", fmt.Errorf("run this from the repo root (no go.mod found in %s)", wd)
	}
	return wd, nil
}
