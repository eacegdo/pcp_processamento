package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wellington/pcp_processamento/internal/domain"
	"github.com/wellington/pcp_processamento/internal/httpapi"
	"github.com/wellington/pcp_processamento/internal/memory"
	"github.com/wellington/pcp_processamento/internal/worker"
)

func TestIngestProgramadoAplicaEJobSuccess(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	body := `[
		{"data":"18/08/2026","fase":"4.2","regional":"NE-I","uf":"BA","inep":"12345678","fornecedor_nome":"NUH","fornecedor_cnpj":"12.345.678/0001-99","quantidade":1,"provisoria":false}
	]`
	id := postProgramado(t, srv, body)
	drain(w)

	got, ok := pcp.GetProgramado(dia(2026, 8, 18), "12345678")
	if !ok {
		t.Fatal("expected Registro PCP programado")
	}
	if got.Tipo != domain.TipoProgramado {
		t.Fatalf("tipo = %q", got.Tipo)
	}
	if got.Quantidade != 1 || got.UF != "BA" || got.RegionalNome != "Nordeste I" {
		t.Fatalf("got %+v", got)
	}
	if got.Provisoria == nil || *got.Provisoria {
		t.Fatalf("provisoria = %v, want false", got.Provisoria)
	}
	job, _ := jobs.Get(id)
	if job.Status != "success" || job.Total != 1 {
		t.Fatalf("job = %+v", job)
	}
	if job.Tipo != domain.TipoProgramado {
		t.Fatalf("job tipo = %q, want programado", job.Tipo)
	}
}

func TestIngestProgramadoAceitaEnvelopeItensEINEPNumerico(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	body := `{"itens":[{"data":"2026-08-18","fase":"4.2","regional":"NO","inep":29443709,"fornecedor_nome":"X"}]}`
	postProgramado(t, srv, body)
	drain(w)

	got, ok := pcp.GetProgramado(dia(2026, 8, 18), "29443709")
	if !ok || got.Quantidade != 1 || got.RegionalNome != "Norte" {
		t.Fatalf("got ok=%v %+v", ok, got)
	}
}

func TestIngestProgramadoDuplicataINEPUltimaVence(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	body := `[
		{"data":"18/08/2026","fase":"4.2","regional":"NE-I","inep":"1","fornecedor_nome":"A"},
		{"data":"18/08/2026","fase":"4.2","regional":"NE-I","inep":"1","fornecedor_nome":"B","provisoria":true}
	]`
	id := postProgramado(t, srv, body)
	drain(w)

	got, _ := pcp.GetProgramado(dia(2026, 8, 18), "1")
	if got.FornecedorNome != "B" || got.Provisoria == nil || !*got.Provisoria {
		t.Fatalf("got %+v, want última ocorrência B provisoria", got)
	}
	job, _ := jobs.Get(id)
	if job.Total != 1 {
		t.Fatalf("total = %d, want 1", job.Total)
	}
}

func TestIngestProgramadoEspelhoDoMesRemoveOmitidos(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 1, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	postProgramado(t, srv, `[
		{"data":"10/08/2026","fase":"4.2","regional":"NE-I","inep":"1","fornecedor_nome":"A"},
		{"data":"18/08/2026","fase":"4.2","regional":"NE-I","inep":"2","fornecedor_nome":"A"},
		{"data":"20/08/2026","fase":"4.2","regional":"NE-I","inep":"3","fornecedor_nome":"A"}
	]`)
	drain(w)

	postProgramado(t, srv, `[
		{"data":"18/08/2026","fase":"4.2","regional":"NE-I","inep":"2","fornecedor_nome":"B"},
		{"data":"20/08/2026","fase":"4.2","regional":"NE-I","inep":"3","fornecedor_nome":"B"}
	]`)
	drain(w)

	if _, ok := pcp.GetProgramado(dia(2026, 8, 10), "1"); ok {
		t.Fatal("Programado omitido do mês deve sair")
	}
	got, ok := pcp.GetProgramado(dia(2026, 8, 18), "2")
	if !ok || got.FornecedorNome != "B" {
		t.Fatalf("inep 2 = ok=%v %+v, want B", ok, got)
	}
	if _, ok := pcp.GetProgramado(dia(2026, 8, 20), "3"); !ok {
		t.Fatal("inep 3 da carga deve permanecer")
	}
}

func TestIngestProgramadoEspelhoNaoMexoOutroMesNemPlanejado(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	postCarga(t, srv, "data,fase,regional,fornecedor,cnpj,quantidade\n"+
		"18/08/2026,4.2,NE-I,NUH,12.345.678/0001-99,10\n")
	drain(w)
	postProgramado(t, srv, `[{"data":"15/07/2026","fase":"4.2","regional":"NO","inep":"9","fornecedor_nome":"Jul"}]`)
	drain(w)

	postProgramado(t, srv, `[
		{"data":"18/08/2026","fase":"4.2","regional":"NE-I","inep":"2","fornecedor_nome":"Ago"},
		{"data":"01/09/2026","fase":"4.2","regional":"NE-I","inep":"8","fornecedor_nome":"Set"}
	]`)
	drain(w)

	if _, ok := pcp.GetProgramado(dia(2026, 7, 15), "9"); !ok {
		t.Fatal("Programado de outro mês deve permanecer")
	}
	if _, ok := pcp.GetProgramado(dia(2026, 8, 18), "2"); !ok {
		t.Fatal("Programado de agosto da carga deve existir")
	}
	if _, ok := pcp.GetProgramado(dia(2026, 9, 1), "8"); ok {
		t.Fatal("item de outro mês na carga não deve gravar")
	}
	got, ok := pcp.Get(dia(2026, 8, 18), "4.2", "NE-I", "12.345.678/0001-99")
	if !ok || got.Quantidade != 10 {
		t.Fatalf("Planejado = ok=%v %+v", ok, got)
	}
}

func TestIngestProgramadoJSONInvalidoNaoApagaMes(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	postProgramado(t, srv, `[{"data":"18/08/2026","fase":"4.2","regional":"NE-I","inep":"1"}]`)
	drain(w)

	req := httptest.NewRequest(http.MethodPost, "/v1/programado", strings.NewReader(`[]`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", testAPIKey)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if _, ok := pcp.GetProgramado(dia(2026, 8, 18), "1"); !ok {
		t.Fatal("JSON sem válidos não deve apagar Programado")
	}
}

func TestIngestProgramadoJSONInvalidoERejeitado(t *testing.T) {
	jobs := memory.NewJobStore()
	srv := httpapi.NewServer(testAPIKey, jobs)

	req := httptest.NewRequest(http.MethodPost, "/v1/programado", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", testAPIKey)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if jobs.Count() != 0 {
		t.Fatalf("jobs = %d", jobs.Count())
	}
}

func TestIngestProgramadoSemAPIKeyERejeitado(t *testing.T) {
	jobs := memory.NewJobStore()
	srv := httpapi.NewServer(testAPIKey, jobs)
	req := httptest.NewRequest(http.MethodPost, "/v1/programado", strings.NewReader(`[]`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func postProgramado(t *testing.T, srv http.Handler, body string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/programado", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", testAPIKey)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp.ID
}
