package server

import (
    "net/url"
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
        // Be permissive on headers to avoid preflight surprises from browsers/frameworks
        AllowHeaders:     "*",
        MaxAge:           3600,
    }
    // If the list contains wildcard-like entries (e.g., https://*.berjis.tech), Fiber's
    // native AllowOrigins won't match them. Provide a suffix-aware function.
    allows := parseAllowed(allow)
    if hasWildcardPattern(allows) {
        cfg.AllowOrigins = ""
        cfg.AllowOriginsFunc = func(origin string) bool {
            o := strings.TrimSpace(origin)
            if o == "" { return false }
            for _, pat := range allows {
                if pat == "*" { return true }
                if pat == o { return true }
                if strings.HasPrefix(pat, "https://*.") || strings.HasPrefix(pat, "http://*.") {
                    // match by suffix of host
                    if hostSuffixMatch(o, pat) { return true }
                }
            }
            return false
        }
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

func parseAllowed(s string) []string {
    parts := strings.Split(s, ",")
    out := make([]string, 0, len(parts))
    for _, p := range parts {
        v := strings.TrimSpace(p)
        if v != "" { out = append(out, v) }
    }
    if len(out) == 0 { out = append(out, "https://berjis.tech") }
    return out
}

func hasWildcardPattern(list []string) bool {
    for _, v := range list {
        if strings.Contains(v, "*.") || v == "*" { return true }
    }
    return false
}

func hostSuffixMatch(origin, pattern string) bool {
    // pattern like https://*.berjis.tech or http://*.example.com
    // compare scheme, then suffix of host
    u, err := url.Parse(origin)
    if err != nil { return false }
    pu, err2 := url.Parse(pattern)
    if err2 != nil { return false }
    if pu.Scheme != "" && u.Scheme != pu.Scheme { return false }
    ph := strings.TrimPrefix(pu.Host, "*.")
    if ph == "" { return false }
    h := u.Host
    // strip port if present
    if i := strings.Index(h, ":"); i >= 0 { h = h[:i] }
    return strings.HasSuffix(h, ph)
}
