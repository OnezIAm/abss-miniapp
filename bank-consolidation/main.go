package main

import (
	"bank-consolidation/internal/config"
	"bank-consolidation/internal/routes"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
)

//go:embed frontend/out frontend/out/_next
var frontendEmbed embed.FS

func main() {
	cfg := config.New()
	db := initDB(cfg.MySQLDSN())

	// Create a sub-filesystem for the frontend/out directory
	frontendFS, err := fs.Sub(frontendEmbed, "frontend/out")
	if err != nil {
		log.Fatalf("Failed to create frontend file system: %v", err)
	}

	engine := routes.Register(db, frontendFS)
	addr := cfg.Addr
	if env := os.Getenv("ADDR"); env != "" {
		addr = env
	}
	srv := &http.Server{Addr: addr, Handler: engine}
	log.Printf("listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
}
