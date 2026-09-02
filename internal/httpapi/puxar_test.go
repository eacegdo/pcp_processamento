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
	rec := postPuxar(t, srv, `{"mes":"2026-08","env":"test"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestPuxarProgramadoEnvInvalidoRetorna400(t *testing.T) {
	srv := httpapi.NewServer(testAPIKey, memory.NewJobStore()).WithBubble(bubble.NewClient("http://127.0.0.1", "tok", nil))
	rec := postPuxar(t, srv, `{"mes":"2026-08","env":"staging"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPuxarProgramadoLiveSemClienteRetorna503(t *testing.T) {
	srv := httpapi.NewServer(testAPIKey, memory.NewJobStore()).WithBubble(bubble.NewClient("http://127.0.0.1", "tok", nil))
	rec := postPuxar(t, srv, `{"mes":"2026-08","env":"live"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestPuxarProgramadoEnfileiraJobEAplica(t *testing.T) {
	bubbleSrv := fakeBubblePuxar(t)
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs).WithBubble(bubble.NewClient(bubbleSrv.URL, "tok", bubbleSrv.Client()))

	rec := postPuxar(t, srv, `{"mes":"2026-08","env":"test"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ID     string `json:"id"`
		Tipo   string `json:"tipo"`
		Itens  int    `json:"itens"`
		Origem string `json:"origem"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Tipo != "programado" || resp.Itens != 1 || resp.Origem != "version-test" {
		t.Fatalf("%+v", resp)
	}
	drain(w)

	got, ok := pcp.GetProgramado(dia(2026, 8, 8), "15026868")
	if !ok || got.Fase != "3" || got.Regional != "NO" || got.Quantidade != 1 {
		t.Fatalf("ok=%v %+v", ok, got)
	}
	if got.Origem != "version-test" {
		t.Fatalf("origem = %q", got.Origem)
	}
	job, _ := jobs.Get(resp.ID)
	if job.Status != "success" {
		t.Fatalf("job = %+v", job)
	}
}

func TestPuxarProgramadoLiveGravaOrigemLive(t *testing.T) {
	bubbleSrv := fakeBubblePuxar(t)
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs).WithBubbleEnv("live", bubble.NewClient(bubbleSrv.URL, "tok", bubbleSrv.Client()))

	rec := postPuxar(t, srv, `{"mes":"2026-08","env":"live"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	drain(w)

	got, ok := pcp.GetProgramado(dia(2026, 8, 8), "15026868")
	if !ok || got.Origem != "live" {
		t.Fatalf("ok=%v origem=%q", ok, got.Origem)
	}
}

// Ponta a ponta do caminho novo: o único item do mês existe porque a Escola
// conectou nele; a Folha de Registro vem de um mês anterior.
func TestPuxarProgramadoItemSoPorConexao(t *testing.T) {
	bubbleSrv := fakeBubbleSoConexao(t)
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs).WithBubble(bubble.NewClient(bubbleSrv.URL, "tok", bubbleSrv.Client()))

	rec := postPuxar(t, srv, `{"mes":"2026-08","env":"test"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ID     string `json:"id"`
		Itens  int    `json:"itens"`
		Resumo struct {
			PorPrevisao int `json:"osps_por_previsao"`
			PorConexao  int `json:"osps_por_conexao"`
		} `json:"resumo"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Itens != 1 || resp.Resumo.PorPrevisao != 0 || resp.Resumo.PorConexao != 1 {
		t.Fatalf("%+v", resp)
	}
	drain(w)

	got, ok := pcp.GetProgramado(dia(2026, 8, 5), "15026868")
	if !ok || got.Fase != "3" || got.Regional != "NO" || got.Quantidade != 1 {
		t.Fatalf("ok=%v %+v", ok, got)
	}
	if _, ainda := pcp.GetProgramado(dia(2026, 7, 10), "15026868"); ainda {
		t.Fatal("gravou também na previsão de julho")
	}
	job, _ := jobs.Get(resp.ID)
	if job.Status != "success" {
		t.Fatalf("job = %+v", job)
	}
}

func fakeBubbleSoConexao(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		cons := r.URL.Query().Get("constraints")
		switch {
		case r.URL.Path == "/obj/osp" && strings.Contains(cons, "Previsão de entrega"):
			_, _ = io.WriteString(w, `{"response":{"cursor":0,"remaining":0,"results":[]}}`)
		case r.URL.Path == "/obj/osp":
			_, _ = io.WriteString(w, `{"response":{"cursor":0,"remaining":0,"results":[{
			  "_id":"osp1","status":"Nota Fiscal","OSnum":13,
			  "Previsão de entrega":"2026-07-10T17:44:00.000Z","FR":["fr1"]
			}]}}`)
		case strings.Contains(r.URL.Path, "import"):
			_, _ = io.WriteString(w, `{"response":{"cursor":0,"remaining":0,"results":[{
			  "inep":"15026868","data_relatorio":"2026-08-05T17:55:00.000Z"
			}]}}`)
		case r.URL.Path == "/obj/fr_osp":
			_, _ = io.WriteString(w, `{"response":{"cursor":0,"remaining":0,"results":[{
			  "_id":"fr1","INEP":"15026868","UF":"PA","Escola":"esc1","OSP":"osp1",
			  "lista de contratos_instalação":["c1"]
			}]}}`)
		case r.URL.Path == "/obj/contrato_taxa_instalacao":
			_, _ = io.WriteString(w, `{"response":{"cursor":0,"remaining":0,"results":[{
			  "_id":"c1","Descrição":"Kit Cobertura Wi-Fi",
			  "Tipo de obra":"4-IMPLANTAÇÃO_DE_REDE_INTERNA"
			}]}}`)
		case r.URL.Path == "/obj/escolas":
			_, _ = io.WriteString(w, `{"response":{"cursor":0,"remaining":0,"results":[{
			  "_id":"esc1","INEP":"15026868","UF":"PA","FASE":"3","Regional":"Norte",
			  "Status Geral":"Conectada","fornecedor_ri":"Q13 TECNOLOGIA",
			  "cnpj_fornecedor_ri":"30.161.238/0001-60"
			}]}}`)
		default:
			t.Errorf("path inesperado %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func postPuxar(t *testing.T, srv http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/programado/puxar", strings.NewReader(body))
	req.Header.Set("X-API-Key", testAPIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func fakeBubblePuxar(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		case r.URL.Path == "/obj/fr_osp":
			_, _ = io.WriteString(w, `{
			  "response": {"cursor":0,"remaining":0,"results":[{
			    "_id":"fr1","INEP":"15026868","UF":"PA",
			    "Escola":"esc1","OSP":"osp1",
			    "lista de contratos_instalação":["c1"]
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
		case r.URL.Path == "/obj/contrato_taxa_instalacao":
			_, _ = io.WriteString(w, `{
			  "response": {"cursor":0,"remaining":0,"results":[{
			    "_id":"c1","Descrição":"Kit Cobertura Wi-Fi",
			    "Tipo de obra":"4-IMPLANTAÇÃO_DE_REDE_INTERNA"
			  }]}
			}`)
		case strings.HasSuffix(r.URL.Path, "/contrato_taxa_instalacao/c1"):
			_, _ = io.WriteString(w, `{
			  "response": {
			    "_id":"c1","Descrição":"Kit Cobertura Wi-Fi",
			    "Tipo de obra":"4-IMPLANTAÇÃO_DE_REDE_INTERNA"
			  }
			}`)
		case r.URL.Path == "/obj/escolas":
			_, _ = io.WriteString(w, `{
			  "response": {
			    "cursor":0,"remaining":0,"results":[{
			    "_id":"esc1","INEP":"15026868","UF":"PA","FASE":"3",
			    "Regional":"Norte","Status Geral":"Em planejamento",
			    "fornecedor_ri":"Q13 TECNOLOGIA",
			    "cnpj_fornecedor_ri":"30.161.238/0001-60"
			  }]
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
		case strings.Contains(r.URL.Path, "import"):
			_, _ = io.WriteString(w, `{"response":{"cursor":0,"remaining":0,"results":[]}}`)
		default:
			t.Errorf("path inesperado %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}
