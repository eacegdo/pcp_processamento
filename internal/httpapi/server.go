package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/wellington/oce_processamento/internal/lote"
	"github.com/wellington/oce_processamento/internal/memory"
	"github.com/wellington/oce_processamento/internal/worker"
)

type Server struct {
	apiKey  string
	jobs    *memory.JobStore
	worker  *worker.Worker
	mux     *http.ServeMux
}

func NewServer(apiKey string, jobs *memory.JobStore, w *worker.Worker) *Server {
	s := &Server{apiKey: apiKey, jobs: jobs, worker: w, mux: http.NewServeMux()}
	s.mux.HandleFunc("POST /v1/lotes", s.handleIngestLote)
	return s
}

func (s *Server) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	s.mux.ServeHTTP(rw, req)
}

func (s *Server) handleIngestLote(rw http.ResponseWriter, req *http.Request) {
	if req.Header.Get("X-API-Key") != s.apiKey {
		http.Error(rw, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	file, header, err := req.FormFile("file")
	if err != nil {
		http.Error(rw, `{"error":"arquivo obrigatório"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	items, err := lote.ParseCSV(file)
	if err != nil {
		http.Error(rw, `{"error":"csv inválido"}`, http.StatusBadRequest)
		return
	}

	fileName := ""
	if header != nil {
		fileName = header.Filename
	}
	job := s.jobs.Create(len(items), fileName, items)

	// Deterministic processing for the HTTP seam tests (issue 01).
	// Issue 03 will move this to an async FIFO loop.
	s.worker.ProcessNext()

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(rw).Encode(map[string]string{"id": job.ID})
}
