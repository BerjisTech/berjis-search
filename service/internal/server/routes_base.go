package server

import (
	"github.com/gofiber/fiber/v2"

	spec "github.com/berjistech/berjis-ecosystem/search/service/openapi"
)

func registerBaseRoutes(app *fiber.App) {
	app.Get("/v1/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true, "message": "ok"})
	})

	app.Get("/openapi/v1.yaml", func(c *fiber.Ctx) error {
		c.Type("yaml")
		return c.Send(spec.Spec)
	})

	app.Get("/openapi", func(c *fiber.Ctx) error {
		html := `<!doctype html>
<html>
  <head>
    <meta charset="utf-8"/>
    <title>Berjis Search API - OpenAPI</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  </head>
  <body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script>
      window.onload = () => {
        window.ui = SwaggerUIBundle({ url: '/openapi/v1.yaml', dom_id: '#swagger-ui', presets: [SwaggerUIBundle.presets.apis] });
      };
    </script>
  </body>
</html>`
		c.Type("html")
		return c.SendString(html)
	})
}
