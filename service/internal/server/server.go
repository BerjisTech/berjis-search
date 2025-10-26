package server

import (
    "encoding/json"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "github.com/gofiber/fiber/v2"
    spec "github.com/berjistech/berjis-ecosystem/search/service/openapi"
    "github.com/berjistech/berjis-ecosystem/search/service/internal/searchindex"
    "github.com/berjistech/berjis-ecosystem/search/service/internal/config"
    htmlpkg "html"
    "sync"
    "time"
)

type Options struct {
    AllowedOrigins string
}

func New(opts Options) *fiber.App {
    app := fiber.New()
    cfg := config.Load()
    store := searchindex.NewStore()
    // serialize write operations per vertical to avoid concurrent map writes and debounced snapshots
    var muWeb, muImg, muVid, muNews sync.Mutex
    var dirtyWeb, dirtyImg, dirtyVid, dirtyNews bool
    saveEvery := func() time.Duration { if s := os.Getenv("SAVE_INTERVAL_SEC"); s != "" { if d, err := strconv.Atoi(s); err == nil && d > 0 { return time.Duration(d) * time.Second } }; return 30 * time.Second }()
    go func() {
        t := time.NewTicker(saveEvery)
        defer t.Stop()
        for range t.C {
            muWeb.Lock(); if dirtyWeb { _ = store.Web.Save(filepath.Join(cfg.PersistDir, "web.json")); dirtyWeb = false }; muWeb.Unlock()
            muImg.Lock(); if dirtyImg { _ = store.Images.Save(filepath.Join(cfg.PersistDir, "images.json")); dirtyImg = false }; muImg.Unlock()
            muVid.Lock(); if dirtyVid { _ = store.Videos.Save(filepath.Join(cfg.PersistDir, "videos.json")); dirtyVid = false }; muVid.Unlock()
            muNews.Lock(); if dirtyNews { _ = store.News.Save(filepath.Join(cfg.PersistDir, "news.json")); dirtyNews = false }; muNews.Unlock()
        }
    }()
    // Load persisted indexes if available
    _ = store.Web.Load(filepath.Join(cfg.PersistDir, "web.json"))
    _ = store.Images.Load(filepath.Join(cfg.PersistDir, "images.json"))
    _ = store.Videos.Load(filepath.Join(cfg.PersistDir, "videos.json"))
    _ = store.News.Load(filepath.Join(cfg.PersistDir, "news.json"))
    // Crawler stats (persisted)
    type HostStat struct{ PagesFetched, BlockedByRobots, CrawlDelaySeconds, SitemapCount int; LastFetch string }
    type CrawlerStats struct{ Hosts map[string]HostStat `json:"hosts"`; Timestamp string `json:"timestamp"` }
    statsPath := filepath.Join(cfg.PersistDir, "crawler-stats.json")
    crawlStats := CrawlerStats{Hosts: map[string]HostStat{}, Timestamp: ""}
    if b, err := os.ReadFile(statsPath); err == nil { _ = json.Unmarshal(b, &crawlStats) }

    // Synonyms dictionary (persisted)
    type Synonyms map[string][]string
    synPath := filepath.Join(cfg.PersistDir, "synonyms.json")
    synMap := Synonyms{}
    if b, err := os.ReadFile(synPath); err == nil { _ = json.Unmarshal(b, &synMap) }
    // apply on startup
    searchindex.SetSynonyms(synMap)

    // Domain priors (persisted): suffix -> multiplier
    type Priors map[string]float64
    priorsPath := filepath.Join(cfg.PersistDir, "priors.json")
    priors := Priors{}
    if b, err := os.ReadFile(priorsPath); err == nil { _ = json.Unmarshal(b, &priors) }
    searchindex.SetDomainPriors(priors)
    // Weights (persisted): field -> weight
    type Weights map[string]float64
    weightsPath := filepath.Join(cfg.PersistDir, "weights.json")
    weights := Weights{}
    if b, err := os.ReadFile(weightsPath); err == nil { _ = json.Unmarshal(b, &weights) }
    if len(weights) > 0 { searchindex.SetWeights(weights) }

    // CORS allowlist (reflect origin) for berjis.tech and subdomains
    app.Use(func(c *fiber.Ctx) error {
        origin := c.Get("Origin")
        if origin != "" {
            if origin == "https://berjis.tech" || strings.HasSuffix(origin, ".berjis.tech") {
                c.Set("Access-Control-Allow-Origin", origin)
                c.Set("Vary", "Origin")
                c.Set("Access-Control-Allow-Credentials", "true")
                c.Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
                // reflect requested headers if present to avoid preflight rejections
                reqHdrs := strings.TrimSpace(c.Get("Access-Control-Request-Headers"))
                if reqHdrs == "" { reqHdrs = "Authorization,Content-Type,Accept" }
                c.Set("Access-Control-Allow-Headers", reqHdrs)
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

    // Seed demo documents into in-memory index
    app.Post("/v1/admin/index/seed", func(c *fiber.Ctx) error {
        demo := []searchindex.Doc{
            { ID: "1", Title: "Berjis – Unified Ecosystem", Url: "https://berjis.tech", Snippet: "Suite of interconnected apps with single sign-on.", Source: "berjis.tech" },
            { ID: "2", Title: "Logistics", Url: "https://logistics.berjis.tech", Snippet: "Logistics platform for supply chain actors.", Source: "logistics.berjis.tech" },
            { ID: "3", Title: "Docs", Url: "https://docs.berjis.tech", Snippet: "Create and collaborate on documents.", Source: "docs.berjis.tech" },
            { ID: "4", Title: "Sheets", Url: "https://sheets.berjis.tech", Snippet: "Powerful spreadsheets for teams.", Source: "sheets.berjis.tech" },
            { ID: "5", Title: "Slides", Url: "https://slides.berjis.tech", Snippet: "Beautiful presentations.", Source: "slides.berjis.tech" },
            { ID: "6", Title: "Notes", Url: "https://notes.berjis.tech", Snippet: "Quick notes synced across devices.", Source: "notes.berjis.tech" },
            { ID: "7", Title: "Communities", Url: "https://communities.berjis.tech", Snippet: "Join discussions and groups.", Source: "communities.berjis.tech" },
            { ID: "8", Title: "Architect", Url: "https://architect.berjis.tech", Snippet: "Design and architecture suite.", Source: "architect.berjis.tech" },
        }
        for _, d := range demo { store.Web.Add(d) }
        _ = store.Web.Save(filepath.Join(cfg.PersistDir, "web.json"))
        return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"seeded": len(demo), "backend": "memory"}})
    })

    // Inspection: stats per vertical
    app.Get("/v1/admin/index/:vertical/stats", func(c *fiber.Ctx) error {
        v := c.Params("vertical")
        var ix *searchindex.Index
        switch v {
        case "web": ix = store.Web
        case "images": ix = store.Images
        case "videos": ix = store.Videos
        case "news": ix = store.News
        default: return c.Status(400).JSON(fiber.Map{"success": false, "message": "unknown vertical"})
        }
        return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"total": ix.Total(), "facets": fiber.Map{"source": ix.FacetSource()}}})
    })
    // Admin: crawler stats get/post
    app.Get("/v1/admin/crawler/stats", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{"success": true, "data": crawlStats})
    })
    app.Post("/v1/admin/crawler/stats", func(c *fiber.Ctx) error {
        var body CrawlerStats
        if err := c.BodyParser(&body); err != nil { return c.Status(400).JSON(fiber.Map{"success": false, "message": "invalid body"}) }
        if body.Hosts == nil { body.Hosts = map[string]HostStat{} }
        crawlStats = body
        // persist
        _ = os.MkdirAll(cfg.PersistDir, 0o755)
        if b, err := json.MarshalIndent(crawlStats, "", "  "); err == nil { _ = os.WriteFile(statsPath, b, 0o644) }
        return c.JSON(fiber.Map{"success": true, "data": crawlStats})
    })

    // Admin: synonyms get/put
    app.Get("/v1/admin/synonyms", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{"success": true, "data": synMap})
    })

    // Admin: domain priors get/put
    app.Get("/v1/admin/priors", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{"success": true, "data": priors})
    })
    app.Put("/v1/admin/priors", func(c *fiber.Ctx) error {
        var body Priors
        if err := c.BodyParser(&body); err != nil { return c.Status(400).JSON(fiber.Map{"success": false, "message": "invalid body"}) }
        // normalize keys
        norm := Priors{}
        for k, v := range body {
            lk := strings.ToLower(strings.TrimSpace(k))
            if lk == "" || v <= 0 { continue }
            norm[lk] = v
        }
        priors = norm
        searchindex.SetDomainPriors(priors)
        _ = os.MkdirAll(cfg.PersistDir, 0o755)
        if b, err := json.MarshalIndent(priors, "", "  "); err == nil { _ = os.WriteFile(priorsPath, b, 0o644) }
        return c.JSON(fiber.Map{"success": true, "data": priors})
    })
    app.Put("/v1/admin/synonyms", func(c *fiber.Ctx) error {
        var body Synonyms
        if err := c.BodyParser(&body); err != nil { return c.Status(400).JSON(fiber.Map{"success": false, "message": "invalid body"}) }
        // normalize keys/values to lowercase
        norm := Synonyms{}
        for k, arr := range body {
            lk := strings.ToLower(strings.TrimSpace(k))
            if lk == "" { continue }
            vals := []string{}
            for _, v := range arr {
                lv := strings.ToLower(strings.TrimSpace(v))
                if lv != "" { vals = append(vals, lv) }
            }
            norm[lk] = vals
        }
        synMap = norm
        searchindex.SetSynonyms(synMap)
        _ = os.MkdirAll(cfg.PersistDir, 0o755)
        if b, err := json.MarshalIndent(synMap, "", "  "); err == nil { _ = os.WriteFile(synPath, b, 0o644) }
        return c.JSON(fiber.Map{"success": true, "data": synMap})
    })
    // Admin: domain priors get/put
    app.Get("/v1/admin/priors", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{"success": true, "data": priors})
    })
    app.Put("/v1/admin/priors", func(c *fiber.Ctx) error {
        var body Priors
        if err := c.BodyParser(&body); err != nil { return c.Status(400).JSON(fiber.Map{"success": false, "message": "invalid body"}) }
        // normalize keys
        norm := Priors{}
        for k, v := range body {
            lk := strings.ToLower(strings.TrimSpace(k))
            if lk == "" || v <= 0 { continue }
            norm[lk] = v
        }
        priors = norm
        searchindex.SetDomainPriors(priors)
        _ = os.MkdirAll(cfg.PersistDir, 0o755)
        if b, err := json.MarshalIndent(priors, "", "  "); err == nil { _ = os.WriteFile(priorsPath, b, 0o644) }
        return c.JSON(fiber.Map{"success": true, "data": priors})
    })
    // Admin: weights get/put
    app.Get("/v1/admin/weights", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{"success": true, "data": weights})
    })
    app.Put("/v1/admin/weights", func(c *fiber.Ctx) error {
        var body Weights
        if err := c.BodyParser(&body); err != nil { return c.Status(400).JSON(fiber.Map{"success": false, "message": "invalid body"}) }
        norm := Weights{}
        for k, v := range body {
            lk := strings.ToLower(strings.TrimSpace(k))
            if lk == "" || v <= 0 { continue }
            norm[lk] = v
        }
        weights = norm
        searchindex.SetWeights(weights)
        _ = os.MkdirAll(cfg.PersistDir, 0o755)
        if b, err := json.MarshalIndent(weights, "", "  "); err == nil { _ = os.WriteFile(weightsPath, b, 0o644) }
        return c.JSON(fiber.Map{"success": true, "data": weights})
    })
    // Inspection: list docs
    app.Get("/v1/admin/index/:vertical/docs", func(c *fiber.Ctx) error {
        v := c.Params("vertical")
        offset, _ := strconv.Atoi(c.Query("offset", "0"))
        limit, _ := strconv.Atoi(c.Query("limit", "50"))
        var ix *searchindex.Index
        switch v {
        case "web": ix = store.Web
        case "images": ix = store.Images
        case "videos": ix = store.Videos
        case "news": ix = store.News
        default: return c.Status(400).JSON(fiber.Map{"success": false, "message": "unknown vertical"})
        }
        return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"items": ix.List(offset, limit), "total": ix.Total()}})
    })
    // Removal: delete by id
    app.Delete("/v1/admin/index/:vertical/:id", func(c *fiber.Ctx) error {
        v := c.Params("vertical")
        id := c.Params("id")
        var ix *searchindex.Index
        var snap string
        switch v {
        case "web": ix = store.Web; snap = "web.json"
        case "images": ix = store.Images; snap = "images.json"
        case "videos": ix = store.Videos; snap = "videos.json"
        case "news": ix = store.News; snap = "news.json"
        default: return c.Status(400).JSON(fiber.Map{"success": false, "message": "unknown vertical"})
        }
        ok := ix.Remove(id)
        if !ok { return c.Status(404).JSON(fiber.Map{"success": false, "message": "not found"}) }
        _ = ix.Save(filepath.Join(cfg.PersistDir, snap))
        return c.JSON(fiber.Map{"success": true})
    })

    // Clear by filters: source and/or host
    app.Post("/v1/admin/index/:vertical/clear", func(c *fiber.Ctx) error {
        v := c.Params("vertical")
        source := c.Query("source", "")
        host := c.Query("host", "")
        var ix *searchindex.Index
        var snap string
        switch v {
        case "web": ix = store.Web; snap = "web.json"
        case "images": ix = store.Images; snap = "images.json"
        case "videos": ix = store.Videos; snap = "videos.json"
        case "news": ix = store.News; snap = "news.json"
        default: return c.Status(400).JSON(fiber.Map{"success": false, "message": "unknown vertical"})
        }
        n := ix.ClearBy(source, host)
        _ = ix.Save(filepath.Join(cfg.PersistDir, snap))
        return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"cleared": n}})
    })

    // Export snapshot (JSON)
    app.Get("/v1/admin/index/:vertical/export", func(c *fiber.Ctx) error {
        v := c.Params("vertical")
        var ix *searchindex.Index
        switch v {
        case "web": ix = store.Web
        case "images": ix = store.Images
        case "videos": ix = store.Videos
        case "news": ix = store.News
        default: return c.Status(400).JSON(fiber.Map{"success": false, "message": "unknown vertical"})
        }
        docs := ix.ExportDocs()
        return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"docs": docs, "total": len(docs)}})
    })

    // Import snapshot (JSON)
    app.Post("/v1/admin/index/:vertical/import", func(c *fiber.Ctx) error {
        v := c.Params("vertical")
        var ix *searchindex.Index
        var snap string
        switch v {
        case "web": ix = store.Web; snap = "web.json"
        case "images": ix = store.Images; snap = "images.json"
        case "videos": ix = store.Videos; snap = "videos.json"
        case "news": ix = store.News; snap = "news.json"
        default: return c.Status(400).JSON(fiber.Map{"success": false, "message": "unknown vertical"})
        }
        // Accept either {docs:[...]} or just [...] array
        var body struct{ Docs []searchindex.Doc `json:"docs"` }
        if err := c.BodyParser(&body); err != nil {
            // try raw array
            var arr []searchindex.Doc
            if err2 := c.BodyParser(&arr); err2 != nil {
                return c.Status(400).JSON(fiber.Map{"success": false, "message": "invalid body"})
            }
            ix.ReplaceAll(arr)
        } else {
            ix.ReplaceAll(body.Docs)
        }
        _ = ix.Save(filepath.Join(cfg.PersistDir, snap))
        return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"total": ix.Total()}})
    })

    // Reindex: rebuild postings from either disk (default) or current memory
    // Body optional: { from: "disk" | "memory" }
    app.Post("/v1/admin/index/:vertical/reindex", func(c *fiber.Ctx) error {
        v := c.Params("vertical")
        var ix *searchindex.Index
        var snap string
        switch v {
        case "web": ix = store.Web; snap = "web.json"
        case "images": ix = store.Images; snap = "images.json"
        case "videos": ix = store.Videos; snap = "videos.json"
        case "news": ix = store.News; snap = "news.json"
        default: return c.Status(400).JSON(fiber.Map{"success": false, "message": "unknown vertical"})
        }
        var body struct{ From string `json:"from"` }
        _ = c.BodyParser(&body)
        from := strings.ToLower(strings.TrimSpace(body.From))
        if from == "memory" {
            docs := ix.ExportDocs()
            ix.ReplaceAll(docs)
        } else {
            // default: disk
            if err := ix.Load(filepath.Join(cfg.PersistDir, snap)); err != nil {
                return c.Status(500).JSON(fiber.Map{"success": false, "message": "load error"})
            }
        }
        // persist rebuilt snapshot
        _ = ix.Save(filepath.Join(cfg.PersistDir, snap))
        return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"total": ix.Total(), "from": map[bool]string{true:"memory", false:"disk"}[from=="memory"]}})
    })

    type BulkWebDoc struct { ID, Title, Url, Snippet, Body, Headings, Source, Date, Lang string }
    type BulkWeb struct { Docs []BulkWebDoc `json:"docs"` }
    app.Post("/v1/admin/index/web", func(c *fiber.Ctx) error {
        muWeb.Lock(); defer muWeb.Unlock()
        var body BulkWeb
        if err := c.BodyParser(&body); err != nil { return c.Status(400).JSON(fiber.Map{"success": false, "message": "invalid body"}) }
        for _, d := range body.Docs {
            store.Web.Add(searchindex.Doc{ID: d.ID, Title: d.Title, Url: d.Url, Snippet: d.Snippet, Body: d.Body, Headings: d.Headings, Source: d.Source, Date: d.Date, Lang: d.Lang})
        }
        dirtyWeb = true
        return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"upserted": len(body.Docs)}})
    })
    type ImageDoc struct { ID, Title, ThumbnailUrl, ImageUrl, Source, Date string }
    type BulkImages struct { Docs []ImageDoc `json:"docs"` }
    app.Post("/v1/admin/index/images", func(c *fiber.Ctx) error {
        muImg.Lock(); defer muImg.Unlock()
        var body BulkImages
        if err := c.BodyParser(&body); err != nil { return c.Status(400).JSON(fiber.Map{"success": false, "message": "invalid body"}) }
        for _, d := range body.Docs { store.Images.Add(searchindex.Doc{ID: d.ID, Title: d.Title, Url: d.ImageUrl, Snippet: d.ThumbnailUrl, Source: d.Source, Date: d.Date}) }
        dirtyImg = true
        return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"upserted": len(body.Docs)}})
    })
    type VideoDoc struct { ID, Title, Url, Duration, ThumbnailUrl, Source, Date string }
    type BulkVideos struct { Docs []VideoDoc `json:"docs"` }
    app.Post("/v1/admin/index/videos", func(c *fiber.Ctx) error {
        muVid.Lock(); defer muVid.Unlock()
        var body BulkVideos
        if err := c.BodyParser(&body); err != nil { return c.Status(400).JSON(fiber.Map{"success": false, "message": "invalid body"}) }
        for _, d := range body.Docs {
            // Store duration in Snippet; store thumbnail URL in Body for later retrieval
            store.Videos.Add(searchindex.Doc{ID: d.ID, Title: d.Title, Url: d.Url, Snippet: d.Duration, Body: d.ThumbnailUrl, Source: d.Source, Date: d.Date})
        }
        dirtyVid = true
        return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"upserted": len(body.Docs)}})
    })
    type NewsDoc struct { ID, Title, Url, Snippet, Source, Date string }
    type BulkNews struct { Docs []NewsDoc `json:"docs"` }
    app.Post("/v1/admin/index/news", func(c *fiber.Ctx) error {
        muNews.Lock(); defer muNews.Unlock()
        var body BulkNews
        if err := c.BodyParser(&body); err != nil { return c.Status(400).JSON(fiber.Map{"success": false, "message": "invalid body"}) }
        for _, d := range body.Docs { store.News.Add(searchindex.Doc{ID: d.ID, Title: d.Title, Url: d.Url, Snippet: d.Snippet, Source: d.Source, Date: d.Date}) }
        dirtyNews = true
        return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"upserted": len(body.Docs)}})
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

        // 'all' → web index search with filters
        if typ == "all" {
            lang := c.Query("lang", "")
            found, total, facets := store.Web.FilteredSearch(q, sourceF, lang, from, to, sort, page, 10)
            type Web struct{ Title, Url, Snippet, SnippetHtml, SnippetPlain, Source string }
            out := make([]Web, 0, len(found))
            langFacets := map[string]int64{}
            for _, r := range found {
                sh := highlight(r.Doc.Snippet, q)
                sp := htmlpkg.UnescapeString(strings.ReplaceAll(strings.ReplaceAll(sh, "<mark>", ""), "</mark>", ""))
                out = append(out, Web{Title: htmlpkg.UnescapeString(r.Doc.Title), Url: r.Doc.Url, Snippet: sh, SnippetHtml: sh, SnippetPlain: sp, Source: r.Doc.Source})
                l := strings.TrimSpace(strings.ToLower(r.Doc.Lang))
                if l == "" { l = "unknown" }
                langFacets[l] = langFacets[l] + 1
            }
            if total == 0 {
                nfound, ntotal, nfacets := store.News.FilteredSearch(q, sourceF, "", from, to, sort, page, 10)
                out = out[:0]
                for _, r := range nfound {
                    sh := highlight(r.Doc.Snippet, q)
                    sp := htmlpkg.UnescapeString(strings.ReplaceAll(strings.ReplaceAll(sh, "<mark>", ""), "</mark>", ""))
                    out = append(out, Web{ Title: htmlpkg.UnescapeString(r.Doc.Title), Url: r.Doc.Url, Snippet: sp, SnippetHtml: sh, SnippetPlain: sp, Source: r.Doc.Source })
                }
                return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sort, "page": page, "total": ntotal, "results": out, "facets": fiber.Map{"source": nfacets, "lang": langFacets}}})
            }
            return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sort, "page": page, "total": total, "results": out, "facets": fiber.Map{"source": facets, "lang": langFacets}}})
        }

        // Placeholder results; swap with real index/metasearch later
        type Web struct{ Title, Url, Snippet, SnippetHtml, SnippetPlain, Source string }
        type Image struct{ Title, ThumbnailUrl, ImageUrl, Source string }
        type Video struct{ Title, Url, Duration, Source string }
        var results any
        var total int

        switch typ {
        case "images":
            found, totalCount, facets := store.Images.FilteredSearch(q, sourceF, "", from, to, sort, page, 30)
            out := make([]Image, 0, len(found))
            for _, r := range found { out = append(out, Image{ Title: r.Doc.Title, ThumbnailUrl: r.Doc.Snippet, ImageUrl: r.Doc.Url, Source: r.Doc.Source }) }
            return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sort, "page": page, "total": totalCount, "results": out, "facets": fiber.Map{"source": facets}}})
        case "videos":
            found, totalCount, facets := store.Videos.FilteredSearch(q, sourceF, "", from, to, sort, page, 10)
            type VideoOut struct{ Title, Url, Duration, ThumbnailUrl, Source string }
            outV := make([]VideoOut, 0, len(found))
            for _, r := range found {
                outV = append(outV, VideoOut{ Title: htmlpkg.UnescapeString(r.Doc.Title), Url: r.Doc.Url, Duration: r.Doc.Snippet, ThumbnailUrl: r.Doc.Body, Source: r.Doc.Source })
            }
            return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sort, "page": page, "total": totalCount, "results": outV, "facets": fiber.Map{"source": facets}}})
        case "news":
            found, totalCount, facets := store.News.FilteredSearch(q, sourceF, "", from, to, sort, page, 10)
            outN := make([]Web, 0, len(found))
            for _, r := range found {
                sh := highlight(r.Doc.Snippet, q)
                sp := htmlpkg.UnescapeString(strings.ReplaceAll(strings.ReplaceAll(sh, "<mark>", ""), "</mark>", ""))
                // For news, default Snippet to plain text to avoid entities when clients render as text
                outN = append(outN, Web{ Title: htmlpkg.UnescapeString(r.Doc.Title), Url: r.Doc.Url, Snippet: sp, SnippetHtml: sh, SnippetPlain: sp, Source: r.Doc.Source })
            }
            return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sort, "page": page, "total": totalCount, "results": outN, "facets": fiber.Map{"source": facets}}})
        case "forums":
            results = []Web{
                { Title: "Communities: Discuss Berjis", Url: "http://communities.berjis.tech", Snippet: "Join conversations about the Berjis ecosystem.", Source: "communities.berjis.tech" },
            }
            total = 1
        case "books":
            results = []Web{
                { Title: "Berjis Books", Url: "http://books.berjis.tech", Snippet: "Explore and organize your digital library.", Source: "books.berjis.tech" },
            }
            total = 1
        case "map":
            results = []Web{
                { Title: "Logistics Map", Url: "https://logistics.berjis.tech", Snippet: "Track deliveries and warehouses.", Source: "logistics.berjis.tech" },
            }
            total = 1
        case "finance":
            results = []Web{
                { Title: "Billing Portal", Url: "https://berjis.tech/account", Snippet: "Manage subscriptions and payments.", Source: "api.berjis.tech" },
            }
            total = 1
        default:
            results = []Web{
                { Title: "Berjis – Unified Ecosystem", Url: "https://berjis.tech", Snippet: "Suite of interconnected apps with single sign-on.", Source: "berjis.tech" },
                { Title: "Logistics", Url: "https://logistics.berjis.tech", Snippet: "Logistics platform for supply chain actors.", Source: "logistics.berjis.tech" },
                { Title: "Docs", Url: "https://docs.berjis.tech", Snippet: "Create and collaborate on documents.", Source: "docs.berjis.tech" },
                { Title: "Sheets", Url: "https://sheets.berjis.tech", Snippet: "Powerful spreadsheets for teams.", Source: "sheets.berjis.tech" },
                { Title: "Slides", Url: "https://slides.berjis.tech", Snippet: "Beautiful presentations in your browser.", Source: "slides.berjis.tech" },
                { Title: "Notes", Url: "https://notes.berjis.tech", Snippet: "Quick notes synced across devices.", Source: "notes.berjis.tech" },
                { Title: "Communities", Url: "https://communities.berjis.tech", Snippet: "Join discussions and groups.", Source: "communities.berjis.tech" },
                { Title: "Architect", Url: "https://architect.berjis.tech", Snippet: "Design and architecture suite.", Source: "architect.berjis.tech" },
            }
            total = 8
        }

        return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sort, "page": page, "total": total, "results": results}})
    })

    return app
}

// highlight applies simple <mark> tags for query terms and phrases in a snippet.
func highlight(snippet string, q string) string {
    if strings.TrimSpace(snippet) == "" { return snippet }
    toks := searchindex.TokenizePublic(q)
    phrases := searchindex.ExtractPhrasesPublic(q)
    words := strings.Fields(snippet)
    hitIdx := -1
    lowerSnippet := strings.ToLower(snippet)
    for _, ph := range phrases {
        if ph == "" { continue }
        if strings.Contains(lowerSnippet, strings.ToLower(ph)) {
            for i, w := range words {
                if strings.Contains(strings.ToLower(w), strings.ToLower(ph)) { hitIdx = i; break }
            }
            if hitIdx != -1 { break }
        }
    }
    if hitIdx == -1 {
        for i, w := range words {
            lw := strings.ToLower(w)
            for _, t := range toks { if t != "" && strings.Contains(lw, t) { hitIdx = i; break } }
            if hitIdx != -1 { break }
        }
    }
    start := 0
    if hitIdx > 0 { start = hitIdx - 12; if start < 0 { start = 0 } }
    end := len(words)
    if start+24 < end { end = start + 24 }
    slice := words[start:end]
    s := strings.Join(slice, " ")
    // Mark placeholders, then escape HTML, then replace placeholders with tags
    const mkStart = "\u0000MK_S\u0000"
    const mkEnd = "\u0000MK_E\u0000"
    for _, ph := range phrases { if ph != "" { s = strings.ReplaceAll(s, ph, mkStart+ph+mkEnd) } }
    for _, tok := range toks { if tok != "" { s = strings.ReplaceAll(s, tok, mkStart+tok+mkEnd) } }
    // Decode any existing HTML entities in snippet before escaping to avoid double-encoding
    s = htmlpkg.UnescapeString(s)
    s = escapeHTML(s)
    s = strings.ReplaceAll(s, escapeHTML(mkStart), "<mark>")
    s = strings.ReplaceAll(s, escapeHTML(mkEnd), "</mark>")
    if start > 0 { s = "… " + s }
    if end < len(words) { s = s + " …" }
    return s
}

func escapeHTML(s string) string {
    r := strings.NewReplacer(
        "&", "&amp;",
        "<", "&lt;",
        ">", "&gt;",
        "\"", "&quot;",
        "'", "&#39;",
    )
    return r.Replace(s)
}

// small adapters to access tokenizer/phrase functions without exporting them
func searchindexExtractPhrases(s string) []string { return searchindex.ExtractPhrasesPublic(s) }
