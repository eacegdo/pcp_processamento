package httpapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wellington/pcp_processamento/internal/bubble"
	"github.com/wellington/pcp_processamento/internal/httpapi"
	"github.com/wellington/pcp_processamento/internal/memory"
	"github.com/wellington/pcp_processamento/internal/worker"
)

func TestPuxarProgramadoSemBubbleRetorna503(t *testing.T) {
	srv := httpapi.NewServer(testAPIKey, memory.NewJobStore())
	req := httptest.NewRequest(http.MethodPost, "/v1/programado/puxar?mes=2026-08", nil)
	req.Header.Set("X-API-Key", testAPIKey)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestPuxarProgramadoEnfileiraJobEAplica(t *testing.T) {
	bubbleSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/obj/osp":
			_, _ = io.WriteString(w, `{
			  "response": {"cursor":0,"remaining":0,"results":[{
			    "_id":"osp1","status":"Nota Fiscal","OSnum":13,
			    "Previsão de entrega":"2026-08-08T17:44:00.000Z",
			    "FR":["fr1"]
			  }]}
			}`)
		case r.URL.Path == "/obj/fr_osp/fr1":
			_, _ = io.WriteString(w, `{
			  "response": {
			    "_id":"fr1","INEP":"15026868","UF":"PA",
			    "Escola":"esc1","OSP":"osp1",
			    "lista de contratos_instalação":["c1"]
			  }
			}`)
		case strings.HasSuffix(r.URL.Path, "/contrato_taxa_instalacao/c1"):
			_, _ = io.WriteString(w, `{
			  "response": {
			    "_id":"c1","Descrição":"Kit Cobertura Wi-Fi",
			    "Tipo de obra":"4-IMPLANTAÇÃO_DE_REDE_INTERNA"
			  }
			}`)
		case r.URL.Path == "/obj/escolas/esc1":
			_, _ = io.WriteString(w, `{
			  "response": {
			    "_id":"esc1","INEP":"15026868","UF":"PA","FASE":"3",
			    "Regional":"Norte","Status Geral":"Em planejamento",
			    "fornecedor_ri":"Q13 TECNOLOGIA",
			    "cnpj_fornecedor_ri":"30.161.238/0001-60"
			  }
			}`)
		default:
			t.Errorf("path inesperado %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(bubbleSrv.Close)

	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs).WithBubble(bubble.NewClient(bubbleSrv.URL, "tok", bubbleSrv.Client()))

	req := httptest.NewRequest(http.MethodPost, "/v1/programado/puxar?mes=2026-08", nil)
	req.Header.Set("X-API-Key", testAPIKey)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ID    string `json:"id"`
		Tipo  string `json:"tipo"`
		Itens int    `json:"itens"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Tipo != "programado" || resp.Itens != 1 {
		t.Fatalf("%+v", resp)
	}
	drain(w)

	got, ok := pcp.GetProgramado(dia(2026, 8, 8), "15026868")
	if !ok || got.Fase != "3" || got.Regional != "NO" || got.Quantidade != 1 {
		t.Fatalf("ok=%v %+v", ok, got)
	}
	job, _ := jobs.Get(resp.ID)
	if job.Status != "success" {
		t.Fatalf("job = %+v", job)
	}
}
