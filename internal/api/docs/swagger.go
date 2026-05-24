package docs

import (
	_ "embed"
	"net/http"
	"text/template"
)

//go:embed openapi.yaml
var OpenAPISpec []byte

const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>dnsmon API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: "{{.SpecURL}}",
      dom_id: '#swagger-ui',
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
      layout: "BaseLayout",
      deepLinking: true,
    });
  </script>
</body>
</html>`

var swaggerTmpl = template.Must(template.New("swagger").Parse(swaggerUIHTML))

type swaggerData struct {
	SpecURL string
}

// SwaggerUI serves the Swagger UI page (wired at GET /api/docs).
func SwaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = swaggerTmpl.Execute(w, swaggerData{SpecURL: "/api/docs/openapi.yaml"})
}

// Spec serves the raw OpenAPI YAML (wired at GET /api/docs/openapi.yaml).
func Spec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(OpenAPISpec)
}
