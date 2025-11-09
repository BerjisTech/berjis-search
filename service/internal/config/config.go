package config

import (
	"os"
)

type Config struct {
	Env            string
	Port           string
	AllowedOrigins string
	PersistDir     string
	CoreAPIBase    string
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func Load() Config {
	return Config{
		Env:            getenv("APP_ENV", "development"),
		Port:           getenv("PORT", "8092"),
		AllowedOrigins: getenv("ALLOWED_ORIGINS", "https://berjis.tech,https://*.berjis.tech"),
		PersistDir:     getenv("PERSIST_DIR", "/data/search"),
		CoreAPIBase:    getenv("CORE_API_BASE", "http://localhost:8080"),
	}
}
