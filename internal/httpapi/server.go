package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/wellington/pcp_processamento/internal/bubble"
	"github.com/wellington/pcp_processamento/internal/domain"
	"github.com/wellington/pcp_processamento/internal/lote"
	"github.com/wellington/pcp_processamento/internal/programado"
)

type Server struct {
	apiKey     string
	jobs       domain.JobStore
	bubbleTest *bubble.Client
	bubbleLive *bubble.Client
	mux        *http.ServeMux
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
	return s.WithBubbleEnv(bubble.AmbienteTest, c)
}

func (s *Server) WithBubbleEnv(env string, c *bubble.Client) *Server {
	amb, err := bubble.Ambiente(env)
	if err != nil {
		return s
	}
	if amb == bubble.AmbienteLive {
		s.bubbleLive = c
		return s
	}
	s.bubbleTest = c
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

type puxarBody struct {
	Mes string `json:"mes"`
	Env string `json:"env"`
}

func (s *Server) handlePuxarProgramado(rw http.ResponseWriter, req *http.Request) {
	if req.Header.Get("X-API-Key") != s.apiKey {
		http.Error(rw, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	rawBody, err := io.ReadAll(io.LimitReader(req.Body, 1<<20))
	if err != nil {
		http.Error(rw, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	var body puxarBody
	if trimmed := bytes.TrimSpace(rawBody); len(trimmed) > 0 {
		if err := json.Unmarshal(trimmed, &body); err != nil {
			http.Error(rw, `{"error":"json inválido"}`, http.StatusBadRequest)
			return
		}
	}

	c, err := s.bubbleDoAmbiente(body.Env)
	if err != nil {
		http.Error(rw, `{"error":"env inválido"}`, http.StatusBadRequest)
		return
	}
	if c == nil {
		http.Error(rw, `{"error":"bubble não configurado"}`, http.StatusServiceUnavailable)
		return
	}

	mes, err := bubble.MesCivil(body.Mes)
	if err != nil {
		http.Error(rw, `{"error":"mês inválido"}`, http.StatusBadRequest)
		return
	}

	got, err := c.PuxarMes(mes)
	if err != nil {
		http.Error(rw, `{"error":"falha ao puxar bubble"}`, http.StatusBadGateway)
		return
	}

	origem, err := bubble.OrigemDoAmbiente(body.Env)
	if err != nil {
		http.Error(rw, `{"error":"env inválido"}`, http.StatusBadRequest)
		return
	}
	for i := range got.Itens {
		got.Itens[i].Origem = origem
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

	amb, _ := bubble.Ambiente(body.Env)
	log.Printf("puxar %s env=%s: %s; %d itens, %d skips", mes.Format("2006-01"), amb, got.Resumo, len(items), len(got.Skips))
	fileName := "puxar-" + mes.Format("2006-01") + "-" + amb + ".json"
	job, err := s.jobs.Create(len(items), domain.TipoProgramado, fileName, items)
	if err != nil {
		http.Error(rw, `{"error":"falha ao criar job"}`, http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(rw).Encode(map[string]any{
		"id":     job.ID,
		"tipo":   job.Tipo,
		"itens":  len(items),
		"skips":  len(got.Skips),
		"origem": origem,
	})
}

func (s *Server) bubbleDoAmbiente(env string) (*bubble.Client, error) {
	amb, err := bubble.Ambiente(env)
	if err != nil {
		return nil, err
	}
	if amb == bubble.AmbienteLive {
		return s.bubbleLive, nil
	}
	return s.bubbleTest, nil
}
