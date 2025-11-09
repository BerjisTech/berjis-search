package server

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/berjistech/berjis-ecosystem/search/service/internal/searchindex"
)

func registerAdminRoutes(app *fiber.App, state *serverState, middleware ...fiber.Handler) {
	admin := app.Group("/v1/admin", middleware...)

	admin.Post("/index/seed", func(c *fiber.Ctx) error {
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

	admin.Get("/index/:vertical/stats", func(c *fiber.Ctx) error {
		v := c.Params("vertical")
		ix, _, _, _, ok := state.resourcesFor(v)
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "unknown vertical"})
		}
		return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"total": ix.Total(), "facets": fiber.Map{"source": ix.FacetSource()}}})
	})

	admin.Get("/crawler/stats", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true, "data": state.crawlStats})
	})

	admin.Post("/crawler/stats", func(c *fiber.Ctx) error {
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

	admin.Get("/synonyms", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true, "data": state.synonyms})
	})

	admin.Put("/synonyms", func(c *fiber.Ctx) error {
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
				if lv := strings.ToLower(strings.TrimSpace(v)); lv != "" {
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

	admin.Get("/priors", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true, "data": state.priors})
	})

	admin.Put("/priors", func(c *fiber.Ctx) error {
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

	admin.Post("/index/upload", func(c *fiber.Ctx) error {
		form, err := c.MultipartForm()
		if err != nil || form == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "invalid form"})
		}
		files := form.File["file"]
		if len(files) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "file required"})
		}
		file := files[0]
		content, err := file.Open()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "failed to open file"})
		}
		defer content.Close()

		backend := strings.ToLower(strings.TrimSpace(c.FormValue("backend")))
		if backend == "" {
			backend = "memory"
		}

		vertical := strings.TrimSpace(c.FormValue("vertical"))
		if vertical == "" {
			vertical = "web"
		}

		index, mu, dirty, snap, ok := state.resourcesFor(vertical)
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "unknown vertical"})
		}
		mu.Lock()
		defer mu.Unlock()

		docs, err := decodeDocs(content)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "invalid document payload"})
		}
		for _, doc := range docs {
			index.Add(doc)
		}
		if dirty != nil {
			*dirty = true
		}
		state.saveIndexSnapshot(index, snap)
		return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"indexed": len(docs), "backend": backend}})
	})

	admin.Post("/index/export", func(c *fiber.Ctx) error {
		target := strings.TrimSpace(c.FormValue("target"))
		if target == "" {
			target = "web"
		}
		ix, _, _, _, ok := state.resourcesFor(target)
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "unknown index"})
		}
		name := "export-" + target + "-" + strconv.FormatInt(time.Now().Unix(), 10) + ".json"
		path := filepath.Join(state.cfg.PersistDir, "exports", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "failed to create export dir"})
		}
		docs := ix.ExportDocs()
		payload, err := json.MarshalIndent(docs, "", "  ")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "failed to encode export"})
		}
		if err := os.WriteFile(path, payload, 0o644); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "failed to write export"})
		}
		return c.JSON(fiber.Map{"success": true, "message": "index exported", "data": fiber.Map{"file": name}})
	})

	admin.Get("/index/export", func(c *fiber.Ctx) error {
		file := strings.TrimSpace(c.Query("file"))
		if file == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "file required"})
		}
		path := filepath.Join(state.cfg.PersistDir, "exports", filepath.Base(file))
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "file not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "failed to stat file"})
		}
		return c.SendFile(path)
	})

	admin.Delete("/index/export", func(c *fiber.Ctx) error {
		file := strings.TrimSpace(c.Query("file"))
		if file == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "file required"})
		}
		path := filepath.Join(state.cfg.PersistDir, "exports", filepath.Base(file))
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "file not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "failed to remove file"})
		}
		return c.JSON(fiber.Map{"success": true})
	})

	admin.Post("/index/:vertical/import", func(c *fiber.Ctx) error {
		vertical := strings.TrimSpace(c.Params("vertical"))
		if vertical == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "vertical required"})
		}
		form, err := c.MultipartForm()
		if err != nil || form == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "invalid form"})
		}
		files := form.File["file"]
		if len(files) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "file required"})
		}
		file := files[0]
		handle, err := file.Open()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "open failed"})
		}
		defer handle.Close()

		backend := strings.ToLower(strings.TrimSpace(c.FormValue("backend")))
		if backend == "" {
			backend = "memory"
		}

		index, mu, dirty, snap, ok := state.resourcesFor(vertical)
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "unknown vertical"})
		}
		mu.Lock()
		defer mu.Unlock()

		docs, err := decodeDocs(handle)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "invalid document payload"})
		}
		index.ReplaceAll(docs)
		if dirty != nil {
			*dirty = true
		}
		state.saveIndexSnapshot(index, snap)
		return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"indexed": len(docs), "backend": backend, "vertical": vertical}})
	})
}

func decodeDocs(r io.Reader) ([]searchindex.Doc, error) {
	payload, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var docs []searchindex.Doc
	if err := json.Unmarshal(payload, &docs); err == nil && len(docs) > 0 {
		return docs, nil
	}
	var wrapper struct {
		Data []searchindex.Doc `json:"data"`
	}
	if err := json.Unmarshal(payload, &wrapper); err == nil && len(wrapper.Data) > 0 {
		return wrapper.Data, nil
	}
	if err := json.Unmarshal(payload, &docs); err == nil {
		return docs, nil
	}
	return nil, errors.New("no documents in payload")
}
