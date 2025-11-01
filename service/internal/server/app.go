package server

import (
	"github.com/gofiber/fiber/v2"

	"github.com/berjistech/berjis-ecosystem/search/service/internal/config"
	"github.com/berjistech/berjis-ecosystem/search/service/internal/searchindex"
)

type Options struct {
	AllowedOrigins string
}

func New(opts Options) *fiber.App {
	app := fiber.New()
	cfg := config.Load()
	store := searchindex.NewStore()
	state := newServerState(cfg, store)

	applyCORS(app, opts.AllowedOrigins)

	registerBaseRoutes(app)
	registerAdminRoutes(app, state)
	registerSearchRoutes(app, state)

	return app
}
