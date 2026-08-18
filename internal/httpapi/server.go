package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/wellington/pcp_processamento/internal/domain"
	"github.com/wellington/pcp_processamento/internal/lote"
	"github.com/wellington/pcp_processamento/internal/programado"
)

type Server struct {
	apiKey string
	jobs   domain.JobStore
	mux    *http.ServeMux
}

func NewServer(apiKey string, jobs domain.JobStore) *Server {
	s := &Server{apiKey: apiKey, jobs: jobs, mux: http.NewServeMux()}
	s.mux.HandleFunc("POST /v1/planejamento", s.handleIngestCarga)
	s.mux.HandleFunc("POST /v1/programado", s.handleIngestProgramado)
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
	job, err := s.jobs.Create(len(items), domain.TipoPlanejado, fileName, items)
	if err != nil {
		http.Error(rw, `{"error":"falha ao criar job"}`, http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(rw).Encode(map[string]string{"id": job.ID, "tipo": job.Tipo})
}

const maxJSONBytes = 16 << 20

func (s *Server) handleIngestProgramado(rw http.ResponseWriter, req *http.Request) {
	if req.Header.Get("X-API-Key") != s.apiKey {
		http.Error(rw, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	body := io.LimitReader(req.Body, maxJSONBytes+1)
	raw, err := io.ReadAll(body)
	if err != nil {
		http.Error(rw, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	if len(raw) > maxJSONBytes {
		http.Error(rw, `{"error":"json grande demais"}`, http.StatusRequestEntityTooLarge)
		return
	}

	items, err := programado.ParseJSON(bytes.NewReader(raw))
	if err != nil {
		http.Error(rw, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}

	job, err := s.jobs.Create(len(items), domain.TipoProgramado, "programado.json", items)
	if err != nil {
		http.Error(rw, `{"error":"falha ao criar job"}`, http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(rw).Encode(map[string]string{"id": job.ID, "tipo": job.Tipo})
}
