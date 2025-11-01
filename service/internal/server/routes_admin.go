package server

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/berjistech/berjis-ecosystem/search/service/internal/searchindex"
)

func registerAdminRoutes(app *fiber.App, state *serverState) {
	app.Post("/v1/admin/index/seed", func(c *fiber.Ctx) error {
		demo := []searchindex.Doc{
			{ID: "1", Title: "Berjis - Unified Ecosystem", Url: "https://berjis.tech", Snippet: "Suite of interconnected apps with single sign-on.", Source: "berjis.tech"},
			{ID: "2", Title: "Logistics", Url: "https://logistics.berjis.tech", Snippet: "Logistics platform for supply chain actors.", Source: "logistics.berjis.tech"},
			{ID: "3", Title: "Docs", Url: "https://docs.berjis.tech", Snippet: "Create and collaborate on documents.", Source: "docs.berjis.tech"},
			{ID: "4", Title: "Sheets", Url: "https://sheets.berjis.tech", Snippet: "Powerful spreadsheets for teams.", Source: "sheets.berjis.tech"},
			{ID: "5", Title: "Slides", Url: "https://slides.berjis.tech", Snippet: "Beautiful presentations.", Source: "slides.berjis.tech"},
			{ID: "6", Title: "Notes", Url: "https://notes.berjis.tech", Snippet: "Quick notes synced across devices.", Source: "notes.berjis.tech"},
			{ID: "7", Title: "Communities", Url: "https://communities.berjis.tech", Snippet: "Join discussions and groups.", Source: "communities.berjis.tech"},
			{ID: "8", Title: "Architect", Url: "https://architect.berjis.tech", Snippet: "Design and architecture suite.", Source: "architect.berjis.tech"},
		}

		ix, mu, dirty, snap, _ := state.resourcesFor("web")
		mu.Lock()
		defer mu.Unlock()
		for _, d := range demo {
			ix.Add(d)
		}
		state.saveIndexSnapshot(ix, snap)
		if dirty != nil {
			*dirty = false
		}
		return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"seeded": len(demo), "backend": "memory"}})
	})

	app.Get("/v1/admin/index/:vertical/stats", func(c *fiber.Ctx) error {
		v := c.Params("vertical")
		ix, _, _, _, ok := state.resourcesFor(v)
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "unknown vertical"})
		}
		return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"total": ix.Total(), "facets": fiber.Map{"source": ix.FacetSource()}}})
	})

	app.Get("/v1/admin/crawler/stats", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true, "data": state.crawlStats})
	})

	app.Post("/v1/admin/crawler/stats", func(c *fiber.Ctx) error {
		var body CrawlerStats
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "invalid body"})
		}
		if body.Hosts == nil {
			body.Hosts = map[string]HostStat{}
		}
		state.crawlStats = body
		state.saveCrawlerStats()
		return c.JSON(fiber.Map{"success": true, "data": state.crawlStats})
	})

	app.Get("/v1/admin/synonyms", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true, "data": state.synonyms})
	})

	app.Put("/v1/admin/synonyms", func(c *fiber.Ctx) error {
		var body Synonyms
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "invalid body"})
		}

		norm := Synonyms{}
		for k, arr := range body {
			lk := strings.ToLower(strings.TrimSpace(k))
			if lk == "" {
				continue
			}
			values := []string{}
			for _, v := range arr {
				lv := strings.ToLower(strings.TrimSpace(v))
				if lv != "" {
					values = append(values, lv)
				}
			}
			norm[lk] = values
		}
		state.synonyms = norm
		searchindex.SetSynonyms(state.synonyms)
		state.saveSynonyms()
		return c.JSON(fiber.Map{"success": true, "data": state.synonyms})
	})

	app.Get("/v1/admin/priors", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true, "data": state.priors})
	})

	app.Put("/v1/admin/priors", func(c *fiber.Ctx) error {
		var body Priors
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "invalid body"})
		}

		norm := Priors{}
		for k, v := range body {
			lk := strings.ToLower(strings.TrimSpace(k))
			if lk == "" || v <= 0 {
				continue
			}
			norm[lk] = v
		}
		state.priors = norm
		searchindex.SetDomainPriors(state.priors)
		state.savePriors()
		return c.JSON(fiber.Map{"success": true, "data": state.priors})
	})

	app.Get("/v1/admin/weights", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true, "data": state.weights})
	})

	app.Put("/v1/admin/weights", func(c *fiber.Ctx) error {
		var body Weights
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "invalid body"})
		}

		norm := Weights{}
		for k, v := range body {
			lk := strings.ToLower(strings.TrimSpace(k))
			if lk == "" || v <= 0 {
				continue
			}
			norm[lk] = v
		}
		state.weights = norm
		if len(state.weights) > 0 {
			searchindex.SetWeights(state.weights)
		}
		state.saveWeights()
		return c.JSON(fiber.Map{"success": true, "data": state.weights})
	})

	app.Get("/v1/admin/index/:vertical/docs", func(c *fiber.Ctx) error {
		v := c.Params("vertical")
		ix, _, _, _, ok := state.resourcesFor(v)
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "unknown vertical"})
		}
		offset, _ := strconv.Atoi(c.Query("offset", "0"))
		limit, _ := strconv.Atoi(c.Query("limit", "50"))
		return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"items": ix.List(offset, limit), "total": ix.Total()}})
	})

	app.Delete("/v1/admin/index/:vertical/:id", func(c *fiber.Ctx) error {
		v := c.Params("vertical")
		id := c.Params("id")
		ix, mu, dirty, snap, ok := state.resourcesFor(v)
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "unknown vertical"})
		}
		mu.Lock()
		defer mu.Unlock()

		if !ix.Remove(id) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "not found"})
		}
		state.saveIndexSnapshot(ix, snap)
		if dirty != nil {
			*dirty = false
		}
		return c.JSON(fiber.Map{"success": true})
	})

	app.Post("/v1/admin/index/:vertical/clear", func(c *fiber.Ctx) error {
		v := c.Params("vertical")
		source := c.Query("source", "")
		host := c.Query("host", "")
		ix, mu, dirty, snap, ok := state.resourcesFor(v)
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "unknown vertical"})
		}
		mu.Lock()
		defer mu.Unlock()

		n := ix.ClearBy(source, host)
		state.saveIndexSnapshot(ix, snap)
		if dirty != nil {
			*dirty = false
		}
		return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"cleared": n}})
	})

	app.Get("/v1/admin/index/:vertical/export", func(c *fiber.Ctx) error {
		v := c.Params("vertical")
		ix, _, _, _, ok := state.resourcesFor(v)
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "unknown vertical"})
		}
		docs := ix.ExportDocs()
		return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"docs": docs, "total": len(docs)}})
	})

	app.Post("/v1/admin/index/:vertical/import", func(c *fiber.Ctx) error {
		v := c.Params("vertical")
		ix, mu, dirty, snap, ok := state.resourcesFor(v)
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "unknown vertical"})
		}
		var body struct {
			Docs  []searchindex.Doc `json:"docs"`
			Items []searchindex.Doc `json:"items"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "invalid body"})
		}
		docs := body.Docs
		if len(docs) == 0 {
			docs = body.Items
		}
		if len(docs) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "no documents provided"})
		}
		mu.Lock()
		defer mu.Unlock()

		ix.ReplaceAll(docs)
		state.saveIndexSnapshot(ix, snap)
		if dirty != nil {
			*dirty = false
		}
		return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"imported": len(docs)}})
	})

	app.Post("/v1/admin/index/:vertical/reindex", func(c *fiber.Ctx) error {
		v := c.Params("vertical")
		ix, mu, dirty, snap, ok := state.resourcesFor(v)
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "unknown vertical"})
		}
		var body struct {
			From string `json:"from"`
		}
		_ = c.BodyParser(&body)
		from := strings.ToLower(strings.TrimSpace(body.From))

		mu.Lock()
		defer mu.Unlock()

		if from == "memory" {
			docs := ix.ExportDocs()
			ix.ReplaceAll(docs)
		} else {
			if err := ix.Load(state.snapshotPath(snap)); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "load error"})
			}
		}
		state.saveIndexSnapshot(ix, snap)
		if dirty != nil {
			*dirty = false
		}
		return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"total": ix.Total(), "from": map[bool]string{true: "memory", false: "disk"}[from == "memory"]}})
	})

	type BulkWebDoc struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Url      string `json:"url"`
		Snippet  string `json:"snippet"`
		Body     string `json:"body"`
		Headings string `json:"headings"`
		Source   string `json:"source"`
		Date     string `json:"date"`
		Lang     string `json:"lang"`
	}
	type BulkWeb struct {
		Docs []BulkWebDoc `json:"docs"`
	}
	app.Post("/v1/admin/index/web", func(c *fiber.Ctx) error {
		ix, mu, dirty, _, _ := state.resourcesFor("web")
		var body BulkWeb
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "invalid body"})
		}
		mu.Lock()
		defer mu.Unlock()
		for _, d := range body.Docs {
			ix.Add(searchindex.Doc{ID: d.ID, Title: d.Title, Url: d.Url, Snippet: d.Snippet, Body: d.Body, Headings: d.Headings, Source: d.Source, Date: d.Date, Lang: d.Lang})
		}
		if dirty != nil {
			*dirty = true
		}
		return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"upserted": len(body.Docs)}})
	})

	type ImageDoc struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		Thumbnail string `json:"thumbnailUrl"`
		ImageURL  string `json:"imageUrl"`
		Source    string `json:"source"`
		Date      string `json:"date"`
	}
	type BulkImages struct {
		Docs []ImageDoc `json:"docs"`
	}
	app.Post("/v1/admin/index/images", func(c *fiber.Ctx) error {
		ix, mu, dirty, _, _ := state.resourcesFor("images")
		var body BulkImages
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "invalid body"})
		}
		mu.Lock()
		defer mu.Unlock()
		for _, d := range body.Docs {
			ix.Add(searchindex.Doc{ID: d.ID, Title: d.Title, Url: d.ImageURL, Snippet: d.Thumbnail, Source: d.Source, Date: d.Date})
		}
		if dirty != nil {
			*dirty = true
		}
		return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"upserted": len(body.Docs)}})
	})

	type VideoDoc struct {
		ID           string `json:"id"`
		Title        string `json:"title"`
		Url          string `json:"url"`
		Duration     string `json:"duration"`
		ThumbnailUrl string `json:"thumbnailUrl"`
		Source       string `json:"source"`
		Date         string `json:"date"`
	}
	type BulkVideos struct {
		Docs []VideoDoc `json:"docs"`
	}
	app.Post("/v1/admin/index/videos", func(c *fiber.Ctx) error {
		ix, mu, dirty, _, _ := state.resourcesFor("videos")
		var body BulkVideos
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "invalid body"})
		}
		mu.Lock()
		defer mu.Unlock()
		for _, d := range body.Docs {
			ix.Add(searchindex.Doc{ID: d.ID, Title: d.Title, Url: d.Url, Snippet: d.Duration, Body: d.ThumbnailUrl, Source: d.Source, Date: d.Date})
		}
		if dirty != nil {
			*dirty = true
		}
		return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"upserted": len(body.Docs)}})
	})

	type NewsDoc struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Url     string `json:"url"`
		Snippet string `json:"snippet"`
		Source  string `json:"source"`
		Date    string `json:"date"`
	}
	type BulkNews struct {
		Docs []NewsDoc `json:"docs"`
	}
	app.Post("/v1/admin/index/news", func(c *fiber.Ctx) error {
		ix, mu, dirty, _, _ := state.resourcesFor("news")
		var body BulkNews
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "invalid body"})
		}
		mu.Lock()
		defer mu.Unlock()
		for _, d := range body.Docs {
			ix.Add(searchindex.Doc{ID: d.ID, Title: d.Title, Url: d.Url, Snippet: d.Snippet, Source: d.Source, Date: d.Date})
		}
		if dirty != nil {
			*dirty = true
		}
		return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"upserted": len(body.Docs)}})
	})
}
