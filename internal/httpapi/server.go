package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/wellington/pcp_processamento/internal/bubble"
	"github.com/wellington/pcp_processamento/internal/domain"
	"github.com/wellington/pcp_processamento/internal/lote"
	"github.com/wellington/pcp_processamento/internal/programado"
)

type Server struct {
	apiKey string
	jobs   domain.JobStore
	bubble *bubble.Client
	mux    *http.ServeMux
}

func NewServer(apiKey string, jobs domain.JobStore) *Server {
	s := &Server{apiKey: apiKey, jobs: jobs, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /", s.handleHealth)
	s.mux.HandleFunc("POST /v1/planejamento", s.handleIngestCarga)
	s.mux.HandleFunc("POST /v1/programado", s.handleIngestProgramado)
	s.mux.HandleFunc("POST /v1/programado/puxar", s.handlePuxarProgramado)
	return s
}

func (s *Server) WithBubble(c *bubble.Client) *Server {
	s.bubble = c
	return s
}

func (s *Server) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	s.mux.ServeHTTP(rw, req)
}

func (s *Server) handleHealth(rw http.ResponseWriter, _ *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(map[string]string{"status": "ok"})
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

func (s *Server) handlePuxarProgramado(rw http.ResponseWriter, req *http.Request) {
	if req.Header.Get("X-API-Key") != s.apiKey {
		http.Error(rw, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if s.bubble == nil {
		http.Error(rw, `{"error":"bubble não configurado"}`, http.StatusServiceUnavailable)
		return
	}

	mes, err := bubble.MesCivil(req.URL.Query().Get("mes"))
	if err != nil {
		http.Error(rw, `{"error":"mês inválido"}`, http.StatusBadRequest)
		return
	}

	got, err := s.bubble.PuxarMes(mes)
	if err != nil {
		http.Error(rw, `{"error":"falha ao puxar bubble"}`, http.StatusBadGateway)
		return
	}

	raw, err := bubble.EncodeProgramadoJSON(got.Itens)
	if err != nil {
		http.Error(rw, `{"error":"json inválido"}`, http.StatusInternalServerError)
		return
	}
	items, err := programado.ParseJSON(bytes.NewReader(raw))
	if err != nil {
		http.Error(rw, `{"error":"nenhum programado no mês"}`, http.StatusBadRequest)
		return
	}

	fileName := "puxar-" + mes.Format("2006-01") + ".json"
	job, err := s.jobs.Create(len(items), domain.TipoProgramado, fileName, items)
	if err != nil {
		http.Error(rw, `{"error":"falha ao criar job"}`, http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(rw).Encode(map[string]any{
		"id":    job.ID,
		"tipo":  job.Tipo,
		"itens": len(items),
		"skips": len(got.Skips),
	})
}
