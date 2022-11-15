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

	//
	//
	//

	s := &http.Server{
		Addr:              cfg.CustomerListen,
		ReadHeaderTimeout: 3 * time.Second,
		Handler:           http.NewServeMux(),
	}

	mux := http.NewServeMux()
	s.Handler = mux

	server.SetupHTTPCustomerServer(mux, cfg)

	log.Fatal(s.ListenAndServe())
}
