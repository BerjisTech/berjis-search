package server

import (
	htmlpkg "html"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func registerSearchRoutes(app *fiber.App, state *serverState) {
	app.Get("/v1/search", func(c *fiber.Ctx) error {
		q := strings.TrimSpace(c.Query("q"))
		typ := strings.ToLower(strings.TrimSpace(c.Query("type", "all")))
		if typ == "" {
			typ = "all"
		}
		sortBy := strings.ToLower(strings.TrimSpace(c.Query("sort", "relevance")))
		page, _ := strconv.Atoi(c.Query("page", "1"))
		if page < 1 {
			page = 1
		}
		sourceF := strings.TrimSpace(c.Query("source"))
		lang := strings.TrimSpace(c.Query("lang"))
		from := strings.TrimSpace(c.Query("from"))
		to := strings.TrimSpace(c.Query("to"))

		type Web struct {
			Title        string `json:"title"`
			Url          string `json:"url"`
			Snippet      string `json:"snippet"`
			SnippetHtml  string `json:"snippetHtml"`
			SnippetPlain string `json:"snippetPlain"`
			Source       string `json:"source"`
		}
		type Image struct {
			Title        string `json:"title"`
			ThumbnailUrl string `json:"thumbnailUrl"`
			ImageUrl     string `json:"imageUrl"`
			Source       string `json:"source"`
		}
		type VideoOut struct {
			Title        string `json:"title"`
			Url          string `json:"url"`
			Duration     string `json:"duration"`
			ThumbnailUrl string `json:"thumbnailUrl"`
			Source       string `json:"source"`
		}

		defaultResults := []Web{
			{Title: "Berjis - Unified Ecosystem", Url: "https://berjis.tech", Snippet: "Suite of interconnected apps with single sign-on.", SnippetHtml: "Suite of interconnected apps with single sign-on.", SnippetPlain: "Suite of interconnected apps with single sign-on.", Source: "berjis.tech"},
			{Title: "Logistics", Url: "https://logistics.berjis.tech", Snippet: "Logistics platform for supply chain actors.", SnippetHtml: "Logistics platform for supply chain actors.", SnippetPlain: "Logistics platform for supply chain actors.", Source: "logistics.berjis.tech"},
			{Title: "Docs", Url: "https://docs.berjis.tech", Snippet: "Create and collaborate on documents.", SnippetHtml: "Create and collaborate on documents.", SnippetPlain: "Create and collaborate on documents.", Source: "docs.berjis.tech"},
			{Title: "Sheets", Url: "https://sheets.berjis.tech", Snippet: "Powerful spreadsheets for teams.", SnippetHtml: "Powerful spreadsheets for teams.", SnippetPlain: "Powerful spreadsheets for teams.", Source: "sheets.berjis.tech"},
			{Title: "Slides", Url: "https://slides.berjis.tech", Snippet: "Beautiful presentations in your browser.", SnippetHtml: "Beautiful presentations in your browser.", SnippetPlain: "Beautiful presentations in your browser.", Source: "slides.berjis.tech"},
			{Title: "Notes", Url: "https://notes.berjis.tech", Snippet: "Quick notes synced across devices.", SnippetHtml: "Quick notes synced across devices.", SnippetPlain: "Quick notes synced across devices.", Source: "notes.berjis.tech"},
			{Title: "Communities", Url: "https://communities.berjis.tech", Snippet: "Join discussions and groups.", SnippetHtml: "Join discussions and groups.", SnippetPlain: "Join discussions and groups.", Source: "communities.berjis.tech"},
			{Title: "Architect", Url: "https://architect.berjis.tech", Snippet: "Design and architecture suite.", SnippetHtml: "Design and architecture suite.", SnippetPlain: "Design and architecture suite.", Source: "architect.berjis.tech"},
		}

		if q == "" {
			return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": "", "type": typ, "sort": sortBy, "page": page, "total": len(defaultResults), "results": defaultResults}})
		}

		switch typ {
		case "all":
			found, total, facets := state.store.Web.FilteredSearch(q, sourceF, lang, from, to, sortBy, page, 10)
			out := make([]Web, 0, len(found))
			langFacets := map[string]int64{}
			for _, r := range found {
				sh := highlight(r.Doc.Snippet, q)
				sp := htmlpkg.UnescapeString(strings.ReplaceAll(strings.ReplaceAll(sh, "<mark>", ""), "</mark>", ""))
				out = append(out, Web{
					Title:        htmlpkg.UnescapeString(r.Doc.Title),
					Url:          r.Doc.Url,
					Snippet:      sh,
					SnippetHtml:  sh,
					SnippetPlain: sp,
					Source:       r.Doc.Source,
				})
				l := strings.TrimSpace(strings.ToLower(r.Doc.Lang))
				if l == "" {
					l = "unknown"
				}
				langFacets[l] = langFacets[l] + 1
			}
			if total == 0 {
				nfound, ntotal, nfacets := state.store.News.FilteredSearch(q, sourceF, "", from, to, sortBy, page, 10)
				out = out[:0]
				for _, r := range nfound {
					sh := highlight(r.Doc.Snippet, q)
					sp := htmlpkg.UnescapeString(strings.ReplaceAll(strings.ReplaceAll(sh, "<mark>", ""), "</mark>", ""))
					out = append(out, Web{
						Title:        htmlpkg.UnescapeString(r.Doc.Title),
						Url:          r.Doc.Url,
						Snippet:      sp,
						SnippetHtml:  sh,
						SnippetPlain: sp,
						Source:       r.Doc.Source,
					})
				}
				return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sortBy, "page": page, "total": ntotal, "results": out, "facets": fiber.Map{"source": nfacets, "lang": langFacets}}})
			}
			return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sortBy, "page": page, "total": total, "results": out, "facets": fiber.Map{"source": facets, "lang": langFacets}}})

		case "images":
			found, totalCount, facets := state.store.Images.FilteredSearch(q, sourceF, "", from, to, sortBy, page, 30)
			out := make([]Image, 0, len(found))
			for _, r := range found {
				out = append(out, Image{
					Title:        r.Doc.Title,
					ThumbnailUrl: r.Doc.Snippet,
					ImageUrl:     r.Doc.Url,
					Source:       r.Doc.Source,
				})
			}
			return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sortBy, "page": page, "total": totalCount, "results": out, "facets": fiber.Map{"source": facets}}})

		case "videos":
			found, totalCount, facets := state.store.Videos.FilteredSearch(q, sourceF, "", from, to, sortBy, page, 10)
			out := make([]VideoOut, 0, len(found))
			for _, r := range found {
				out = append(out, VideoOut{
					Title:        htmlpkg.UnescapeString(r.Doc.Title),
					Url:          r.Doc.Url,
					Duration:     r.Doc.Snippet,
					ThumbnailUrl: r.Doc.Body,
					Source:       r.Doc.Source,
				})
			}
			return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sortBy, "page": page, "total": totalCount, "results": out, "facets": fiber.Map{"source": facets}}})

		case "news":
			found, totalCount, facets := state.store.News.FilteredSearch(q, sourceF, "", from, to, sortBy, page, 10)
			out := make([]Web, 0, len(found))
			for _, r := range found {
				sh := highlight(r.Doc.Snippet, q)
				sp := htmlpkg.UnescapeString(strings.ReplaceAll(strings.ReplaceAll(sh, "<mark>", ""), "</mark>", ""))
				out = append(out, Web{
					Title:        htmlpkg.UnescapeString(r.Doc.Title),
					Url:          r.Doc.Url,
					Snippet:      sp,
					SnippetHtml:  sh,
					SnippetPlain: sp,
					Source:       r.Doc.Source,
				})
			}
			return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sortBy, "page": page, "total": totalCount, "results": out, "facets": fiber.Map{"source": facets}}})

		case "forums":
			results := []Web{
				{Title: "Communities: Discuss Berjis", Url: "https://communities.berjis.tech", Snippet: "Join conversations about the Berjis ecosystem.", SnippetHtml: "Join conversations about the Berjis ecosystem.", SnippetPlain: "Join conversations about the Berjis ecosystem.", Source: "communities.berjis.tech"},
			}
			return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sortBy, "page": page, "total": len(results), "results": results}})

		case "books":
			results := []Web{
				{Title: "Berjis Books", Url: "https://books.berjis.tech", Snippet: "Explore and organize your digital library.", SnippetHtml: "Explore and organize your digital library.", SnippetPlain: "Explore and organize your digital library.", Source: "books.berjis.tech"},
			}
			return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sortBy, "page": page, "total": len(results), "results": results}})

		case "map":
			results := []Web{
				{Title: "Logistics Map", Url: "https://logistics.berjis.tech", Snippet: "Track deliveries and warehouses.", SnippetHtml: "Track deliveries and warehouses.", SnippetPlain: "Track deliveries and warehouses.", Source: "logistics.berjis.tech"},
			}
			return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sortBy, "page": page, "total": len(results), "results": results}})

		case "finance":
			results := []Web{
				{Title: "Billing Portal", Url: "https://berjis.tech/account", Snippet: "Manage subscriptions and payments.", SnippetHtml: "Manage subscriptions and payments.", SnippetPlain: "Manage subscriptions and payments.", Source: "api.berjis.tech"},
			}
			return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sortBy, "page": page, "total": len(results), "results": results}})

		default:
			return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"q": q, "type": typ, "sort": sortBy, "page": page, "total": len(defaultResults), "results": defaultResults}})
		}
	})
}
