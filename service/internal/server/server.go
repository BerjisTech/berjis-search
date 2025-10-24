package server

import (
    "log"
    "strconv"
    "strings"

    "github.com/gofiber/fiber/v2"
    spec "github.com/berjistech/berjis-ecosystem/search/service/openapi"
    "github.com/berjistech/berjis-ecosystem/search/service/internal/searchindex"
    "github.com/berjistech/berjis-ecosystem/search/service/internal/config"
    search "github.com/berjistech/berjis-ecosystem/search/service/internal/search"
)

type Options struct {
    AllowedOrigins string
}

func New(opts Options) *fiber.App {
    app := fiber.New()
    ix := searchindex.New()
    cfg := config.Load()
    var meili *search.Meili
    if cfg.MeiliHost != "" {
        meili = search.NewMeili(cfg.MeiliHost, cfg.MeiliAPIKey)
        log.Printf("search: using meilisearch at %s", cfg.MeiliHost)
    }

    // CORS allowlist (reflect origin) for berjis.test and subdomains
    app.Use(func(c *fiber.Ctx) error {
        origin := c.Get("Origin")
        if origin != "" {
            if origin == "http://berjis.test" || strings.HasSuffix(origin, ".berjis.test") {
                c.Set("Access-Control-Allow-Origin", origin)
                c.Set("Vary", "Origin")
                c.Set("Access-Control-Allow-Credentials", "true")
                c.Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
                c.Set("Access-Control-Allow-Headers", "Authorization,Content-Type,Accept")
                if c.Method() == fiber.MethodOptions { return c.SendStatus(fiber.StatusNoContent) }
            }
        }
        return c.Next()
    })

    app.Get("/v1/health", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{"success": true, "message": "ok"})
    })

    // OpenAPI YAML + Swagger UI
    app.Get("/openapi/v1.yaml", func(c *fiber.Ctx) error {
        c.Type("yaml")
        return c.Send(spec.Spec)
    })
    app.Get("/openapi", func(c *fiber.Ctx) error {
        html := `<!doctype html>
<html>
  <head>
    <meta charset="utf-8"/>
    <title>Berjis Search API – OpenAPI</title>
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

    // Seed demo documents into in-memory index or Meili
    app.Post("/v1/admin/index/seed", func(c *fiber.Ctx) error {
        demo := []search.WebDoc{
            { ID: "1", Title: "Berjis – Unified Ecosystem", Url: "http://berjis.test", Snippet: "Suite of interconnected apps with single sign-on.", Source: "berjis.test" },
            { ID: "2", Title: "Logistics", Url: "http://logistics.berjis.test", Snippet: "Logistics platform for supply chain actors.", Source: "logistics.berjis.test" },
            { ID: "3", Title: "Docs", Url: "http://docs.berjis.test", Snippet: "Create and collaborate on documents.", Source: "docs.berjis.test" },
            { ID: "4", Title: "Sheets", Url: "http://sheets.berjis.test", Snippet: "Powerful spreadsheets for teams.", Source: "sheets.berjis.test" },
            { ID: "5", Title: "Slides", Url: "http://slides.berjis.test", Snippet: "Beautiful presentations.", Source: "slides.berjis.test" },
            { ID: "6", Title: "Notes", Url: "http://notes.berjis.test", Snippet: "Quick notes synced across devices.", Source: "notes.berjis.test" },
            { ID: "7", Title: "Communities", Url: "http://communities.berjis.test", Snippet: "Join discussions and groups.", Source: "communities.berjis.test" },
            { ID: "8", Title: "Architect", Url: "http://architect.berjis.test", Snippet: "Design and architecture suite.", Source: "architect.berjis.test" },
        }
        if meili != nil {
            if err := meili.IndexWeb(demo); err != nil {
                return c.Status(500).JSON(fiber.Map{"success": false, "message": err.Error()})
            }
            return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"seeded": len(demo), "backend": "meilisearch"}})
        }
        for _, d := range demo { ix.Add(searchindex.Doc{ID: d.ID, Title: d.Title, Url: d.Url, Snippet: d.Snippet, Source: d.Source}) }
        return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"seeded": len(demo), "backend": "memory"}})
    })

    // Query serving endpoint
    app.Get("/v1/search", func(c *fiber.Ctx) error {
        q := strings.TrimSpace(c.Query("q"))
        typ := c.Query("type", "all")
        sort := c.Query("sort", "relevance")
        pageStr := c.Query("page", "1")
        page, _ := strconv.Atoi(pageStr)
        if page < 1 { page = 1 }
        sourceF := c.Query("source", "")
        from := c.Query("from", "")
        to := c.Query("to", "")
        if q == "" {
            return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sort, "page": page, "total": 0, "results": []any{}}})
        }

        // If searching 'all', try Meili; fallback to in-memory index results
        if typ == "all" {
            if meili != nil {
                wr, err := meili.SearchWeb(q, page, 10, sourceF, from, to, sort)
                if err == nil && len(wr.Hits) > 0 {
                    type Web struct{ Title, Url, Snippet, Source string }
                    out := make([]Web, 0, len(wr.Hits))
                    for _, r := range wr.Hits { out = append(out, Web{ Title: r.Title, Url: r.Url, Snippet: r.Snippet, Source: r.Source }) }
                    return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sort, "page": page, "total": wr.Total, "results": out, "facets": wr.Facets}})
                }
            }
            found := ix.Search(q, 10)
            if len(found) > 0 {
                type Web struct{ Title, Url, Snippet, Source string }
                out := make([]Web, 0, len(found))
                for _, r := range found {
                    out = append(out, Web{ Title: r.Doc.Title, Url: r.Doc.Url, Snippet: r.Doc.Snippet, Source: r.Doc.Source })
                }
                return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sort, "page": page, "total": len(out), "results": out}})
            }
        }

        // Placeholder results; swap with real index/metasearch later
        type Web struct{ Title, Url, Snippet, Source string }
        type Image struct{ Title, ThumbnailUrl, ImageUrl, Source string }
        type Video struct{ Title, Url, Duration, Source string }
        var results any
        var total int

        switch typ {
        case "images":
            if meili != nil {
                ir, err := meili.SearchImages(q, page, 30, sourceF, from, to, sort)
                if err == nil {
                    out := make([]Image, 0, len(ir.Hits))
                    for _, r := range ir.Hits { out = append(out, Image{ Title: r.Title, ThumbnailUrl: r.ThumbnailUrl, ImageUrl: r.ImageUrl, Source: r.Source }) }
                    return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sort, "page": page, "total": ir.Total, "results": out, "facets": ir.Facets}})
                }
            }
            results = []Image{{ Title: "Berjis Logo", ThumbnailUrl: "http://berjis.test/static/logo-128.png", ImageUrl: "http://berjis.test/static/logo.png", Source: "berjis.test" }}
            total = 1
        case "videos":
            if meili != nil {
                vr, err := meili.SearchVideos(q, page, 10, sourceF, from, to, sort)
                if err == nil {
                    out := make([]Video, 0, len(vr.Hits))
                    for _, r := range vr.Hits { out = append(out, Video{ Title: r.Title, Url: r.Url, Duration: r.Duration, Source: r.Source }) }
                    return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sort, "page": page, "total": vr.Total, "results": out, "facets": vr.Facets}})
                }
            }
            results = []Video{{ Title: "Introducing Berjis Logistics", Url: "http://logistics.berjis.test", Duration: "2:03", Source: "logistics.berjis.test" }}
            total = 1
        case "news":
            if meili != nil {
                nr, err := meili.SearchNews(q, page, 10, sourceF, from, to, sort)
                if err == nil {
                    out := make([]Web, 0, len(nr.Hits))
                    for _, r := range nr.Hits { out = append(out, Web{ Title: r.Title, Url: r.Url, Snippet: r.Snippet, Source: r.Source }) }
                    return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sort, "page": page, "total": nr.Total, "results": out, "facets": nr.Facets}})
                }
            }
            results = []Web{{ Title: "Berjis announces Logistics beta", Url: "http://berjis.test", Snippet: "Early access to unified logistics platform.", Source: "berjis.test" }}
            total = 1
        case "forums":
            results = []Web{
                { Title: "Communities: Discuss Berjis", Url: "http://communities.berjis.test", Snippet: "Join conversations about the Berjis ecosystem.", Source: "communities.berjis.test" },
            }
            total = 1
        case "books":
            results = []Web{
                { Title: "Berjis Books", Url: "http://books.berjis.test", Snippet: "Explore and organize your digital library.", Source: "books.berjis.test" },
            }
            total = 1
        case "map":
            results = []Web{
                { Title: "Logistics Map", Url: "http://logistics.berjis.test", Snippet: "Track deliveries and warehouses.", Source: "logistics.berjis.test" },
            }
            total = 1
        case "finance":
            results = []Web{
                { Title: "Billing Portal", Url: "http://berjis.test/account", Snippet: "Manage subscriptions and payments.", Source: "api.berjis.test" },
            }
            total = 1
        default:
            results = []Web{
                { Title: "Berjis – Unified Ecosystem", Url: "http://berjis.test", Snippet: "Suite of interconnected apps with single sign-on.", Source: "berjis.test" },
                { Title: "Logistics", Url: "http://logistics.berjis.test", Snippet: "Logistics platform for supply chain actors.", Source: "logistics.berjis.test" },
                { Title: "Docs", Url: "http://docs.berjis.test", Snippet: "Create and collaborate on documents.", Source: "docs.berjis.test" },
                { Title: "Sheets", Url: "http://sheets.berjis.test", Snippet: "Powerful spreadsheets for teams.", Source: "sheets.berjis.test" },
                { Title: "Slides", Url: "http://slides.berjis.test", Snippet: "Beautiful presentations in your browser.", Source: "slides.berjis.test" },
                { Title: "Notes", Url: "http://notes.berjis.test", Snippet: "Quick notes synced across devices.", Source: "notes.berjis.test" },
                { Title: "Communities", Url: "http://communities.berjis.test", Snippet: "Join discussions and groups.", Source: "communities.berjis.test" },
                { Title: "Architect", Url: "http://architect.berjis.test", Snippet: "Design and architecture suite.", Source: "architect.berjis.test" },
            }
            total = 8
        }

        return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sort, "page": page, "total": total, "results": results}})
    })

    return app
}
