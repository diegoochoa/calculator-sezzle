package httpapi

import (
	"net/http"

	"github.com/diegoochoa/calculator-sezzle-api/api"
)

// The specification is hand-written rather than generated from annotations.
// Generation would mean a code-generator dependency, a build step and a docs
// package in the tree; this API is small enough that one reviewed file is
// cheaper, and openapi_test.go fails the build if it drifts from the routes
// actually served.

// Paths for the specification and the browsable documentation. Exported so the
// router and its tests cannot disagree about them.
const (
	SpecPath = "/v1/openapi.yaml"
	DocsPath = "/docs"
)

// handleOpenAPISpec serves the raw specification, for client generators and
// anything else that consumes OpenAPI directly.
//
// Unauthenticated on purpose: a client has to read this in order to learn how
// to authenticate, and the document describes the API rather than revealing
// anything about its data.
func (s *Server) handleOpenAPISpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(api.Spec())
}

// swaggerUI renders the specification in a browser.
//
// The assets come from a pinned CDN build rather than being vendored: Swagger
// UI is several megabytes and this is a 15 MB distroless image whose whole
// point is having nothing spare in it. The trade-off is that this page needs
// network access; the specification at SpecPath does not, so offline tooling
// and client generation are unaffected.
const swaggerUI = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Calculator API</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css" />
    <style>
      body { margin: 0; background: #fafafa; }
      .swagger-ui .topbar { display: none; }
    </style>
  </head>
  <body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js" crossorigin></script>
    <script>
      window.ui = SwaggerUIBundle({
        url: '` + SpecPath + `',
        dom_id: '#swagger-ui',
        deepLinking: true,
        displayRequestDuration: true,
        defaultModelsExpandDepth: 0,
        tryItOutEnabled: true,
      });
    </script>
  </body>
</html>
`

// handleDocs serves the Swagger UI page.
func (s *Server) handleDocs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write([]byte(swaggerUI))
}
