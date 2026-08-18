package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/diegoochoa/calculator-sezzle-api/api"
	"github.com/diegoochoa/calculator-sezzle-api/internal/calc"
	"gopkg.in/yaml.v3"
)

// A specification nobody checks is decoration. These tests fail the build when
// the document and the server disagree, which is the only thing that makes it
// worth trusting.

type openAPIDocument struct {
	OpenAPI string `yaml:"openapi"`
	Info    struct {
		Title   string `yaml:"title"`
		Version string `yaml:"version"`
	} `yaml:"info"`
	Paths      map[string]map[string]operation `yaml:"paths"`
	Components struct {
		Schemas         map[string]yaml.Node `yaml:"schemas"`
		Responses       map[string]yaml.Node `yaml:"responses"`
		SecuritySchemes map[string]yaml.Node `yaml:"securitySchemes"`
	} `yaml:"components"`
}

type operation struct {
	OperationID string         `yaml:"operationId"`
	Summary     string         `yaml:"summary"`
	Tags        []string       `yaml:"tags"`
	Security    *[]yaml.Node   `yaml:"security"`
	Responses   map[string]any `yaml:"responses"`
}

func loadSpec(t *testing.T) openAPIDocument {
	t.Helper()

	var doc openAPIDocument
	if err := yaml.Unmarshal(api.Spec(), &doc); err != nil {
		t.Fatalf("openapi.yaml is not valid YAML: %v", err)
	}
	return doc
}

func TestSpecIsWellFormed(t *testing.T) {
	t.Parallel()

	doc := loadSpec(t)

	if !strings.HasPrefix(doc.OpenAPI, "3.") {
		t.Errorf("openapi = %q, want a 3.x document", doc.OpenAPI)
	}
	if doc.Info.Title == "" || doc.Info.Version == "" {
		t.Errorf("info is incomplete: %+v", doc.Info)
	}
	if len(doc.Paths) == 0 {
		t.Fatal("the specification documents no paths")
	}
	if _, ok := doc.Components.SecuritySchemes["bearerAuth"]; !ok {
		t.Error("no bearerAuth security scheme, yet every calculation route needs one")
	}

	for path, methods := range doc.Paths {
		for method, op := range methods {
			if op.OperationID == "" {
				t.Errorf("%s %s has no operationId, so generated clients get a name from the path", method, path)
			}
			if op.Summary == "" {
				t.Errorf("%s %s has no summary", method, path)
			}
			if len(op.Tags) == 0 {
				t.Errorf("%s %s is untagged, so it lands in a default group", method, path)
			}
			if len(op.Responses) == 0 {
				t.Errorf("%s %s documents no responses", method, path)
			}
		}
	}
}

// Every documented route must actually exist. A 404 or 405 here means the
// specification promises something the server does not serve.
func TestSpecDocumentsOnlyRealRoutes(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	doc := loadSpec(t)

	for path, methods := range doc.Paths {
		for method := range methods {
			verb := strings.ToUpper(method)

			t.Run(verb+" "+path, func(t *testing.T) {
				body := ""
				if verb == http.MethodPost {
					// Deliberately empty: the route only has to exist, and an
					// empty body is answered with 400 rather than 404.
					body = "{}"
				}

				rec := h.do(t, verb, path, body)
				if rec.Code == http.StatusNotFound {
					t.Errorf("documented but not served (404)")
				}
				if rec.Code == http.StatusMethodNotAllowed {
					t.Errorf("documented for %s but the server does not accept that method", verb)
				}
			})
		}
	}
}

// And the reverse: every route the server serves must be documented, or the
// specification silently goes stale as endpoints are added.
func TestEveryRouteIsDocumented(t *testing.T) {
	t.Parallel()

	doc := loadSpec(t)

	served := []struct{ method, path string }{
		{http.MethodPost, "/v1/auth/token"},
		{http.MethodPost, "/v1/calculate"},
		{http.MethodPost, "/v1/calculate/batch"},
		{http.MethodPost, "/v1/validate"},
		{http.MethodGet, "/v1/functions"},
		{http.MethodGet, "/healthz"},
		{http.MethodGet, "/readyz"},
	}

	for _, route := range served {
		methods, ok := doc.Paths[route.path]
		if !ok {
			t.Errorf("%s is served but absent from the specification", route.path)
			continue
		}
		if _, ok := methods[strings.ToLower(route.method)]; !ok {
			t.Errorf("%s %s is served but not documented", route.method, route.path)
		}
	}
}

// The engine's own error codes are the contract clients switch on, so the
// enumeration in the specification must not fall behind them.
func TestSpecEnumeratesEveryErrorCode(t *testing.T) {
	t.Parallel()

	spec := string(api.Spec())

	codes := []string{
		calc.CodeEmpty, calc.CodeTooLong, calc.CodeSyntax, calc.CodeUnbalancedParen,
		calc.CodeUnknownIdentifier, calc.CodeWrongArity, calc.CodeDepthExceeded,
		calc.CodeDivisionByZero, calc.CodeDomain, calc.CodeUndefined, calc.CodeOverflow,
	}
	for _, code := range codes {
		if !strings.Contains(spec, code) {
			t.Errorf("engine error code %s is not documented", code)
		}
	}

	transport := []string{
		"INVALID_REQUEST", "UNAUTHORIZED", "NOT_FOUND", "METHOD_NOT_ALLOWED",
		"PAYLOAD_TOO_LARGE", "RATE_LIMITED", "TIMEOUT", "INTERNAL_ERROR",
	}
	for _, code := range transport {
		if !strings.Contains(spec, code) {
			t.Errorf("transport error code %s is not documented", code)
		}
	}
}

// Documentation has to be reachable without a token, or a new client cannot
// discover how to obtain one.
func TestDocumentationNeedsNoCredentials(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	spec := h.doWithToken(t, http.MethodGet, SpecPath, "", "")
	if spec.Code != http.StatusOK {
		t.Fatalf("%s = %d, want 200 without a token", SpecPath, spec.Code)
	}
	if got := spec.Header().Get("Content-Type"); !strings.Contains(got, "yaml") {
		t.Errorf("Content-Type = %q, want YAML", got)
	}
	if !strings.Contains(spec.Body.String(), "openapi:") {
		t.Error("the served body is not the specification")
	}

	docs := h.doWithToken(t, http.MethodGet, DocsPath, "", "")
	if docs.Code != http.StatusOK {
		t.Fatalf("%s = %d, want 200 without a token", DocsPath, docs.Code)
	}
	if !strings.Contains(docs.Body.String(), SpecPath) {
		t.Errorf("the docs page does not point at %s", SpecPath)
	}
}

// A dangling $ref is the classic hand-written-specification bug, and it only
// shows up as "could not resolve reference" once someone opens the page.
func TestEveryReferenceResolves(t *testing.T) {
	t.Parallel()

	var root yaml.Node
	if err := yaml.Unmarshal(api.Spec(), &root); err != nil {
		t.Fatalf("openapi.yaml is not valid YAML: %v", err)
	}

	doc := loadSpec(t)
	known := map[string]bool{}
	for name := range doc.Components.Schemas {
		known["#/components/schemas/"+name] = true
	}
	for name := range doc.Components.Responses {
		known["#/components/responses/"+name] = true
	}

	var refs []string
	var walk func(node *yaml.Node)
	walk = func(node *yaml.Node) {
		if node.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(node.Content); i += 2 {
				if node.Content[i].Value == "$ref" {
					refs = append(refs, node.Content[i+1].Value)
				}
				walk(node.Content[i+1])
			}
			return
		}
		for _, child := range node.Content {
			walk(child)
		}
	}
	walk(&root)

	if len(refs) == 0 {
		t.Fatal("found no $ref at all, so this test is not checking anything")
	}
	for _, ref := range refs {
		if !known[ref] {
			t.Errorf("$ref %q does not resolve", ref)
		}
	}
	t.Logf("checked %d references against %d definitions", len(refs), len(known))
}

// The catalog endpoint is what clients build keypads from, so its schema has to
// match what the handler actually emits.
//
// Derived from the live catalog rather than a hardcoded field list: an earlier
// version of this test listed the names by hand, and when constants were
// removed from the engine it went on passing against a specification that still
// documented them.
func TestSpecMatchesTheCatalogShape(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(calc.NewCatalog(calc.Options{}))
	if err != nil {
		t.Fatalf("marshalling the catalog: %v", err)
	}
	var emitted map[string]json.RawMessage
	if err := json.Unmarshal(payload, &emitted); err != nil {
		t.Fatalf("the catalog is not a JSON object: %v", err)
	}

	documented := schemaProperties(t, "Catalog")

	for field := range emitted {
		if _, ok := documented[field]; !ok {
			t.Errorf("the catalog emits %q but the specification does not document it", field)
		}
	}
	for field := range documented {
		if _, ok := emitted[field]; !ok {
			t.Errorf("the specification documents %q but the catalog does not emit it", field)
		}
	}

	// Nested element shapes: every field an entry carries must be described.
	// One direction only, since optional fields are legitimately absent.
	for schema, sample := range map[string]any{
		"FunctionInfo": calc.NewCatalog(calc.Options{}).Functions[0],
		"OperatorInfo": calc.NewCatalog(calc.Options{}).Operators[0],
	} {
		raw, err := json.Marshal(sample)
		if err != nil {
			t.Fatalf("marshalling %s: %v", schema, err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatalf("%s is not a JSON object: %v", schema, err)
		}

		properties := schemaProperties(t, schema)
		for field := range fields {
			if _, ok := properties[field]; !ok {
				t.Errorf("%s emits %q but the specification does not document it", schema, field)
			}
		}
	}
}

// schemaProperties returns the property names a component schema declares.
func schemaProperties(t *testing.T, name string) map[string]yaml.Node {
	t.Helper()

	node, ok := loadSpec(t).Components.Schemas[name]
	if !ok {
		t.Fatalf("schema %s is missing from the specification", name)
	}

	var schema struct {
		Properties map[string]yaml.Node `yaml:"properties"`
	}
	if err := node.Decode(&schema); err != nil {
		t.Fatalf("decoding schema %s: %v", name, err)
	}
	return schema.Properties
}
