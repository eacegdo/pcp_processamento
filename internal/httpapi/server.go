package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/wellington/pcp_processamento/internal/domain"
	"github.com/wellington/pcp_processamento/internal/lote"
)

type Server struct {
	apiKey string
	jobs   domain.JobStore
	mux    *http.ServeMux
}

func NewServer(apiKey string, jobs domain.JobStore) *Server {
	s := &Server{apiKey: apiKey, jobs: jobs, mux: http.NewServeMux()}
	s.mux.HandleFunc("POST /v1/cargas", s.handleIngestCarga)
	return s
}

func (s *Server) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	s.mux.ServeHTTP(rw, req)
}

func (s *Server) handleIngestCarga(rw http.ResponseWriter, req *http.Request) {
	if req.Header.Get("X-API-Key") != s.apiKey {
		http.Error(rw, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	file, header, err := req.FormFile("file")
	if err != nil {
		http.Error(rw, `{"error":"carga obrigatória"}`, http.StatusBadRequest)
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
	job, err := s.jobs.Create(len(items), fileName, items)
	if err != nil {
		http.Error(rw, `{"error":"falha ao criar job"}`, http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(rw).Encode(map[string]string{"id": job.ID})
}
