package server

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/berjistech/berjis-ecosystem/search/service/internal/auth"
	"github.com/berjistech/berjis-ecosystem/search/service/internal/config"
	"github.com/berjistech/berjis-ecosystem/search/service/internal/searchindex"
	coreauth "github.com/berjistech/berjis-ecosystem/shared/coreauth"
)

type Options struct {
	Config config.Config
}

func New(opts Options) *fiber.App {
	app := fiber.New()
	cfg := opts.Config
	store := searchindex.NewStore()
	state := newServerState(cfg, store)

	applyCORS(app, cfg.AllowedOrigins)

	authClient := &http.Client{Timeout: 8 * time.Second}
	var verifier *coreauth.Verifier
	if base := strings.TrimSpace(cfg.CoreAPIBase); base != "" {
		if v, err := coreauth.NewVerifier(coreauth.Config{
			CoreAPIBase: base,
			HTTPClient:  authClient,
		}); err != nil {
			log.Printf("warn: coreauth verifier init failed: %v", err)
		} else {
			verifier = v
		}
	}

	requireAuth := auth.Middleware(auth.Options{
		Env:         cfg.Env,
		CoreAPIBase: cfg.CoreAPIBase,
		HTTPClient:  authClient,
		Verifier:    verifier,
	})
	requireAdmin := auth.RequireRolesMiddleware("platform.admin", "platform.search.admin", "search.admin")

	registerBaseRoutes(app)
	registerAdminRoutes(app, state, requireAuth, requireAdmin)
	registerSearchRoutes(app, state)

	return app
}
