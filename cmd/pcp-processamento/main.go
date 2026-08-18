package main

import (
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"
	"github.com/wellington/pcp_processamento/internal/config"
	"github.com/wellington/pcp_processamento/internal/httpapi"
	"github.com/wellington/pcp_processamento/internal/supabase"
	"github.com/wellington/pcp_processamento/internal/worker"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	jobs := supabase.NewJobStore(cfg.SupabaseURL, cfg.SupabaseServiceRoleKey, nil)
	pcp := supabase.NewPcpStore(cfg.SupabaseURL, cfg.SupabaseServiceRoleKey, nil)
	w := worker.New(jobs, pcp, worker.Config{
		BatchSize:  cfg.BatchSize,
		MaxRetries: cfg.BatchMaxRetries,
	})
	srv := httpapi.NewServer(cfg.APIKey, jobs)

	go runWorker(w)

	log.Printf("ouvindo %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, srv); err != nil {
		log.Fatalf("http: %v", err)
	}
}

func runWorker(w *worker.Worker) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		for w.ProcessNext() {
		}
	}
}
