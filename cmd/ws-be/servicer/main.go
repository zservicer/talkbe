package main

import (
	"log"
	"net/http"
	"time"

	"github.com/zservicer/talkbe/config"
	"github.com/zservicer/talkbe/internal/server"
)

func main() {
	cfg := config.GetWSConfig()

	s := &http.Server{
		Addr:              cfg.ServicerListen,
		ReadHeaderTimeout: 3 * time.Second,
	}

	mux := http.NewServeMux()
	s.Handler = mux

	server.SetupHTTPServicerServer(mux, cfg)
	log.Fatal(s.ListenAndServe())
}
