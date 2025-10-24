package config

import (
    "os"
)

type Config struct {
    Env            string
    Port           string
    AllowedOrigins string
    MeiliHost      string
    MeiliAPIKey    string
}

func getenv(k, def string) string {
    if v := os.Getenv(k); v != "" { return v }
    return def
}

func Load() Config {
    return Config{
        Env: getenv("APP_ENV", "development"),
        Port: getenv("PORT", "8092"),
        AllowedOrigins: getenv("ALLOWED_ORIGINS", "http://berjis.test"),
        MeiliHost: getenv("MEILI_HOST", "http://meilisearch:7700"),
        MeiliAPIKey: getenv("MEILI_API_KEY", "devkey"),
    }
}
