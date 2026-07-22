package main

import (
	"log"
	"net/http"

	"github.com/ruth411/circle/internal/platform/config"
	"github.com/ruth411/circle/internal/platform/httpapi"
)

func main() {
	cfg := config.Load()
	addr := ":" + cfg.Port

	log.Printf("circle listening on %s", addr)
	if err := http.ListenAndServe(addr, httpapi.NewServer()); err != nil {
		log.Fatal(err)
	}
}
