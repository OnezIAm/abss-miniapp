package main

import (
	"bank-consolidation/internal/config"
	"bank-consolidation/internal/routes"
	"log"
	"net/http"
	"os"
)

func main() {
	cfg := config.New()
	db := initDB(cfg.MySQLDSN())
	engine := routes.Register(db)
	addr := cfg.Addr
	if env := os.Getenv("ADDR"); env != "" {
		addr = env
	}
	srv := &http.Server{Addr: addr, Handler: engine}
	log.Printf("listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
}
