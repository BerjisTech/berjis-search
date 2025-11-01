package server

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func applyCORS(app *fiber.App, raw string) {
	allow := normalizeAllowOrigins(raw)
	cfg := cors.Config{
		AllowOrigins:     allow,
		AllowCredentials: true,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Authorization,Content-Type,Accept,X-Requested-With",
		MaxAge:           3600,
	}
	app.Use(cors.New(cfg))
}

func normalizeAllowOrigins(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "https://berjis.tech,https://*.berjis.tech"
	}

	parts := strings.Split(raw, ",")
	seen := make(map[string]struct{}, len(parts))
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if value == "*" {
			return "*"
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, value)
	}

	if len(normalized) == 0 {
		return "https://berjis.tech,https://*.berjis.tech"
	}

	return strings.Join(normalized, ",")
}
