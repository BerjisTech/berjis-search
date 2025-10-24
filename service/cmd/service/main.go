package main

import (
    "log"
    "os"

    "github.com/berjistech/berjis-ecosystem/search/service/internal/config"
    "github.com/berjistech/berjis-ecosystem/search/service/internal/server"
)

func main() {
    cfg := config.Load()
    app := server.New(server.Options{ AllowedOrigins: cfg.AllowedOrigins })
    addr := ":" + cfg.Port
    log.Printf("starting search-api on %s (env=%s)", addr, cfg.Env)
    if err := app.Listen(addr); err != nil {
        log.Println("shutdown:", err)
        os.Exit(1)
    }
}

