package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wellington/pcp_processamento/internal/domain"
	"github.com/wellington/pcp_processamento/internal/httpapi"
	"github.com/wellington/pcp_processamento/internal/memory"
	"github.com/wellington/pcp_processamento/internal/worker"
)

const testAPIKey = "test-api-key"

func TestIngestCargaAutenticadoRetornaIDDoJob(t *testing.T) {
	jobs := memory.NewJobStore()
	srv := httpapi.NewServer(testAPIKey, jobs)

	body, contentType := multipartCSV(t, "carga.csv", minimalCSV())
	req := httptest.NewRequest(http.MethodPost, "/v1/planejamento", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-API-Key", testAPIKey)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID == "" {
		t.Fatal("expected non-empty job id")
	}

	job, ok := jobs.Get(resp.ID)
	if !ok {
		t.Fatalf("job %q not found in store", resp.ID)
	}
	if job.Status != "queued" {
		t.Fatalf("job status = %q, want queued", job.Status)
	}
}

func TestIngestCargaAplicaPlanejadoEJobSuccess(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	id := postCarga(t, srv, minimalCSV())
	drain(w)

	got, ok := pcp.Get(dia(2026, 8, 18), "4.2", "NE-I", "12.345.678/0001-99")
	if !ok {
		t.Fatal("expected Registro PCP")
	}
	if got.Tipo != domain.TipoPlanejado {
		t.Fatalf("tipo = %q, want planejado", got.Tipo)
	}
	if got.Quantidade != 10 {
		t.Fatalf("quantidade = %d, want 10", got.Quantidade)
	}
	if got.FornecedorNome != "NUH DIGITAL" {
		t.Fatalf("fornecedor_nome = %q", got.FornecedorNome)
	}
	if got.RegionalNome != "Nordeste I" {
		t.Fatalf("regional_nome = %q, want Nordeste I", got.RegionalNome)
	}
	if got.INEP != "" || got.UF != "" || got.Provisoria != nil {
		t.Fatalf("colunas de Programado devem ficar vazias: %+v", got)
	}

	job, ok := jobs.Get(id)
	if !ok {
		t.Fatal("job not found")
	}
	if job.Status != "success" {
		t.Fatalf("job status = %q, want success", job.Status)
	}
	if job.Total != 1 || job.Processadas != 1 {
		t.Fatalf("progress total=%d processadas=%d, want 1/1", job.Total, job.Processadas)
	}
}

func TestJobAvancaProcessadasPorBatchAteSuccess(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 2, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	csv := "data,fase,regional,fornecedor,cnpj,quantidade\n" +
		"18/08/2026,4.2,NE-I,A,12.345.678/0001-99,1\n" +
		"19/08/2026,4.2,NE-I,A,12.345.678/0001-99,2\n" +
		"20/08/2026,4.2,NE-I,A,12.345.678/0001-99,3\n"
	id := postCarga(t, srv, csv)

	if !w.ProcessNext() {
		t.Fatal("expected first batch")
	}
	job, _ := jobs.Get(id)
	if job.Status != "running" || job.Processadas != 2 || job.Restantes != 1 {
		t.Fatalf("after first batch status=%q processadas=%d restantes=%d", job.Status, job.Processadas, job.Restantes)
	}

	if !w.ProcessNext() {
		t.Fatal("expected second batch")
	}
	job, _ = jobs.Get(id)
	if job.Status != "success" || job.Processadas != 3 || job.Restantes != 0 {
		t.Fatalf("status=%q processadas=%d restantes=%d, want success 3/0", job.Status, job.Processadas, job.Restantes)
	}
}

func TestSegundaCargaFicaQueuedEnquantoPrimeiraRunning(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 2, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	csv1 := "data,fase,regional,fornecedor,cnpj,quantidade\n" +
		"18/08/2026,4.2,NE-I,A,11.111.111/0001-11,1\n" +
		"19/08/2026,4.2,NE-I,A,11.111.111/0001-11,1\n" +
		"20/08/2026,4.2,NE-I,A,11.111.111/0001-11,1\n"
	id1 := postCarga(t, srv, csv1)
	if !w.ProcessNext() {
		t.Fatal("expected first job to start")
	}
	job1, _ := jobs.Get(id1)
	if job1.Status != "running" {
		t.Fatalf("job1 status = %q, want running", job1.Status)
	}

	csv2 := "data,fase,regional,fornecedor,cnpj,quantidade\n" +
		"18/08/2026,4.2,NO,B,22.222.222/0001-22,5\n"
	id2 := postCarga(t, srv, csv2)
	job2, _ := jobs.Get(id2)
	if job2.Status != "queued" {
		t.Fatalf("job2 status = %q, want queued", job2.Status)
	}

	drain(w)
	job1, _ = jobs.Get(id1)
	job2, _ = jobs.Get(id2)
	if job1.Status != "success" || job2.Status != "success" {
		t.Fatalf("job1=%q job2=%q, want success", job1.Status, job2.Status)
	}
}

func TestFilaFIFOProcessaNaOrdemDeEnqueue(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 1, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	csvFirst := "data,fase,regional,fornecedor,cnpj,quantidade\n" +
		"18/08/2026,4.2,NE-I,A,12.345.678/0001-99,10\n"
	csvSecond := "data,fase,regional,fornecedor,cnpj,quantidade\n" +
		"18/08/2026,4.2,NE-I,A,12.345.678/0001-99,9\n"
	id1 := postCarga(t, srv, csvFirst)
	id2 := postCarga(t, srv, csvSecond)

	if !w.ProcessNext() {
		t.Fatal("expected first queued job")
	}
	job1, _ := jobs.Get(id1)
	job2, _ := jobs.Get(id2)
	if job1.Status != "success" {
		t.Fatalf("job1 status = %q, want success", job1.Status)
	}
	if job2.Status != "queued" {
		t.Fatalf("job2 status = %q, want queued", job2.Status)
	}
	got, _ := pcp.Get(dia(2026, 8, 18), "4.2", "NE-I", "12.345.678/0001-99")
	if got.Quantidade != 10 {
		t.Fatalf("quantidade after first = %d, want 10", got.Quantidade)
	}

	if !w.ProcessNext() {
		t.Fatal("expected second queued job")
	}
	got, _ = pcp.Get(dia(2026, 8, 18), "4.2", "NE-I", "12.345.678/0001-99")
	if got.Quantidade != 9 {
		t.Fatalf("quantidade after second = %d, want 9", got.Quantidade)
	}
}

func TestFalhaTransitoriaDeBatchERetentada(t *testing.T) {
	pcp := memory.NewPcpStore()
	pcp.FailNext(2)
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 1, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	id := postCarga(t, srv, minimalCSV())
	drain(w)

	job, _ := jobs.Get(id)
	if job.Status != "success" {
		t.Fatalf("status = %q error=%q, want success after retries", job.Status, job.ErrorMessage)
	}
	got, _ := pcp.Get(dia(2026, 8, 18), "4.2", "NE-I", "12.345.678/0001-99")
	if got.Quantidade != 10 {
		t.Fatalf("quantidade = %d, want 10", got.Quantidade)
	}
}

func TestFalhaAposRetriesMarcaFailedEPreservaAplicado(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 1, MaxRetries: 2})
	srv := httpapi.NewServer(testAPIKey, jobs)

	csv := "data,fase,regional,fornecedor,cnpj,quantidade\n" +
		"18/08/2026,4.2,NE-I,A,11.111.111/0001-11,1\n" +
		"19/08/2026,4.2,NE-I,A,11.111.111/0001-11,2\n"
	id := postCarga(t, srv, csv)

	if !w.ProcessNext() {
		t.Fatal("expected first batch")
	}
	got1, ok := pcp.Get(dia(2026, 8, 18), "4.2", "NE-I", "11.111.111/0001-11")
	if !ok || got1.Quantidade != 1 {
		t.Fatalf("first row not applied: ok=%v %+v", ok, got1)
	}

	pcp.FailNext(2)
	if !w.ProcessNext() {
		t.Fatal("expected failing batch")
	}
	job, _ := jobs.Get(id)
	if job.Status != "failed" || job.ErrorMessage == "" || job.Processadas != 1 {
		t.Fatalf("job = %+v, want failed with processadas=1", job)
	}
	if _, ok := pcp.Get(dia(2026, 8, 19), "4.2", "NE-I", "11.111.111/0001-11"); ok {
		t.Fatal("second row should not have been applied")
	}
}

func TestAposJobFailedProximoQueuedInicia(t *testing.T) {
	pcp := memory.NewPcpStore()
	pcp.FailNext(2)
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 1, MaxRetries: 2})
	srv := httpapi.NewServer(testAPIKey, jobs)

	idFail := postCarga(t, srv, "data,fase,regional,fornecedor,cnpj,quantidade\n18/08/2026,4.2,NE-I,A,11.111.111/0001-11,1\n")
	idOk := postCarga(t, srv, "data,fase,regional,fornecedor,cnpj,quantidade\n18/08/2026,4.2,NO,B,22.222.222/0001-22,5\n")

	if !w.ProcessNext() {
		t.Fatal("expected failing job")
	}
	jobFail, _ := jobs.Get(idFail)
	if jobFail.Status != "failed" {
		t.Fatalf("jobFail status = %q, want failed", jobFail.Status)
	}
	if !w.ProcessNext() {
		t.Fatal("expected next queued job")
	}
	jobOk, _ := jobs.Get(idOk)
	if jobOk.Status != "success" {
		t.Fatalf("jobOk status = %q, want success", jobOk.Status)
	}
	got, _ := pcp.Get(dia(2026, 8, 18), "4.2", "NO", "22.222.222/0001-22")
	if got.Quantidade != 5 || got.RegionalNome != "Norte" {
		t.Fatalf("segunda carga = %+v", got)
	}
}

func TestIngestCargaSemAPIKeyValidaERejeitado(t *testing.T) {
	for _, tc := range []struct {
		name   string
		apiKey string
	}{
		{name: "ausente", apiKey: ""},
		{name: "errada", apiKey: "chave-errada"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pcp := memory.NewPcpStore()
			jobs := memory.NewJobStore()
			srv := httpapi.NewServer(testAPIKey, jobs)

			body, contentType := multipartCSV(t, "carga.csv", minimalCSV())
			req := httptest.NewRequest(http.MethodPost, "/v1/planejamento", body)
			req.Header.Set("Content-Type", contentType)
			if tc.apiKey != "" {
				req.Header.Set("X-API-Key", tc.apiKey)
			}
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
			if jobs.Count() != 0 {
				t.Fatalf("expected no jobs, got %d", jobs.Count())
			}
			if pcp.Count() != 0 {
				t.Fatalf("expected no registros, got %d", pcp.Count())
			}
		})
	}
}

func TestIngestCargaCSVInvalidoERejeitado(t *testing.T) {
	jobs := memory.NewJobStore()
	srv := httpapi.NewServer(testAPIKey, jobs)

	body, contentType := multipartCSV(t, "carga.csv", "foo,bar\n1,2\n")
	req := httptest.NewRequest(http.MethodPost, "/v1/planejamento", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-API-Key", testAPIKey)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if jobs.Count() != 0 {
		t.Fatalf("expected no jobs, got %d", jobs.Count())
	}
}

func TestIngestCargaDuplicataChaveUltimaOcorrenciaVence(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	csv := "data,fase,regional,fornecedor,cnpj,quantidade\n" +
		"18/08/2026,4.2,NE-I,A,12.345.678/0001-99,10\n" +
		"18/08/2026,4.2,NE-I,B,12.345.678/0001-99,9\n"
	id := postCarga(t, srv, csv)
	drain(w)

	got, _ := pcp.Get(dia(2026, 8, 18), "4.2", "NE-I", "12.345.678/0001-99")
	if got.Quantidade != 9 || got.FornecedorNome != "B" {
		t.Fatalf("got %+v, want quantidade=9 nome=B", got)
	}
	job, _ := jobs.Get(id)
	if job.Total != 1 {
		t.Fatalf("total = %d, want 1 após dedupe", job.Total)
	}
}

func TestIngestCargaCorrigeQuantidadeInclusiveZero(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	postCarga(t, srv, minimalCSV())
	drain(w)
	postCarga(t, srv, "data,fase,regional,fornecedor,cnpj,quantidade\n18/08/2026,4.2,NE-I,NUH DIGITAL,12.345.678/0001-99,0\n")
	drain(w)

	got, ok := pcp.Get(dia(2026, 8, 18), "4.2", "NE-I", "12.345.678/0001-99")
	if !ok {
		t.Fatal("chave existente deve permanecer")
	}
	if got.Quantidade != 0 {
		t.Fatalf("quantidade = %d, want 0", got.Quantidade)
	}
}

func TestIngestCargaChaveNovaComZeroNaoGrava(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	id := postCarga(t, srv, "data,fase,regional,fornecedor,cnpj,quantidade\n18/08/2026,4.2,NE-I,A,12.345.678/0001-99,0\n")
	drain(w)

	if _, ok := pcp.Get(dia(2026, 8, 18), "4.2", "NE-I", "12.345.678/0001-99"); ok {
		t.Fatal("chave nova com zero não deve gravar")
	}
	job, _ := jobs.Get(id)
	if job.Status != "success" || job.Total != 1 || job.Processadas != 1 {
		t.Fatalf("job = %+v, want success total=1 (zero explícito entra no trabalho)", job)
	}
}

func TestIngestCargaNaoApagaChaveOmitidaNoReenvio(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	postCarga(t, srv, "data,fase,regional,fornecedor,cnpj,quantidade\n"+
		"18/08/2026,4.2,NE-I,A,11.111.111/0001-11,10\n"+
		"18/08/2026,4.2,NO,B,22.222.222/0001-22,5\n")
	drain(w)
	postCarga(t, srv, "data,fase,regional,fornecedor,cnpj,quantidade\n"+
		"18/08/2026,4.2,NE-I,A,11.111.111/0001-11,9\n")
	drain(w)

	gotNE, _ := pcp.Get(dia(2026, 8, 18), "4.2", "NE-I", "11.111.111/0001-11")
	gotNO, ok := pcp.Get(dia(2026, 8, 18), "4.2", "NO", "22.222.222/0001-22")
	if gotNE.Quantidade != 9 {
		t.Fatalf("NE-I quantidade = %d, want 9", gotNE.Quantidade)
	}
	if !ok || gotNO.Quantidade != 5 {
		t.Fatalf("NO omitida no reenvio deve permanecer: ok=%v %+v", ok, gotNO)
	}
}

func TestIngestCargaNomeFornecedorOpcionalECNPJComoVeio(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	postCarga(t, srv, "data,fase,regional,fornecedor,cnpj,quantidade\n18/08/2026,4.2,NE-I,,12.345.678/0001-99,3\n")
	drain(w)

	got, ok := pcp.Get(dia(2026, 8, 18), "4.2", "NE-I", "12.345.678/0001-99")
	if !ok {
		t.Fatal("expected registro com CNPJ mascarado")
	}
	if got.FornecedorNome != "" || got.FornecedorCNPJ != "12.345.678/0001-99" {
		t.Fatalf("got %+v", got)
	}
}

func TestIngestCargaRegionalDesconhecidaGravaSiglaSemNome(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	postCarga(t, srv, "data,fase,regional,fornecedor,cnpj,quantidade\n18/08/2026,4.2,NEI,A,12.345.678/0001-99,1\n")
	drain(w)

	got, ok := pcp.Get(dia(2026, 8, 18), "4.2", "NEI", "12.345.678/0001-99")
	if !ok {
		t.Fatal("sigla desconhecida deve entrar")
	}
	if got.Regional != "NEI" || got.RegionalNome != "" {
		t.Fatalf("got %+v, want sigla NEI e nome vazio", got)
	}
}

func TestIngestCargaDeParaRegionaisConhecidas(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	postCarga(t, srv, "data,fase,regional,fornecedor,cnpj,quantidade\n"+
		"18/08/2026,4.2,SUSE,A,11.111.111/0001-11,1\n"+
		"18/08/2026,4.2,COSE,A,22.222.222/0001-22,1\n"+
		"18/08/2026,4.2,NE-II,A,33.333.333/0001-33,1\n")
	drain(w)

	suse, _ := pcp.Get(dia(2026, 8, 18), "4.2", "SUSE", "11.111.111/0001-11")
	cose, _ := pcp.Get(dia(2026, 8, 18), "4.2", "COSE", "22.222.222/0001-22")
	ne2, _ := pcp.Get(dia(2026, 8, 18), "4.2", "NE-II", "33.333.333/0001-33")
	if suse.RegionalNome != "Sudeste/Centro-Sul" {
		t.Fatalf("SUSE nome = %q", suse.RegionalNome)
	}
	if cose.RegionalNome != "Centro-Oeste/Minas" {
		t.Fatalf("COSE nome = %q", cose.RegionalNome)
	}
	if ne2.RegionalNome != "Nordeste II" {
		t.Fatalf("NE-II nome = %q", ne2.RegionalNome)
	}
}

func TestIngestCargaIgnoraLinhaInvalidaEAceitaPontoEVirgula(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	csv := "data;fase;regional;fornecedor;cnpj;quantidade\n" +
		"18/08/2026;4.2;NE-I;A;12.345.678/0001-99;4,34\n" +
		"2026-08-18;4.2;NE-I;A;12.345.678/0001-99;10\n" +
		"18/08/2026;4.2;NE-I;A;;10\n" +
		"18/08/2026;4.2;NE-I;A;12.345.678/0001-99;\n" +
		"18/08/2026;4.2;NE-I;A;12.345.678/0001-99;8\n"
	postCarga(t, srv, csv)
	drain(w)

	got, ok := pcp.Get(dia(2026, 8, 18), "4.2", "NE-I", "12.345.678/0001-99")
	if !ok || got.Quantidade != 8 {
		t.Fatalf("got ok=%v %+v, want quantidade 8 (linha válida; decimal/data ISO/sem CNPJ/qtd vazia ignoradas)", ok, got)
	}
}

func TestIngestCargaColunasForaDeOrdemEHeaderMaiusculo(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	csv := "Quantidade,CNPJ,Regional,Fase,Data,Fornecedor\n" +
		"7,12.345.678/0001-99,NO,5,01/09/2026,ACME\n"
	postCarga(t, srv, csv)
	drain(w)

	got, ok := pcp.Get(dia(2026, 9, 1), "5", "NO", "12.345.678/0001-99")
	if !ok || got.Quantidade != 7 || got.FornecedorNome != "ACME" || got.RegionalNome != "Norte" {
		t.Fatalf("got ok=%v %+v", ok, got)
	}
}

func TestIngestCargaIgnoraQuantidadeDecimalComVirgulaComoDelimitador(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	csv := "data,fase,regional,fornecedor,cnpj,quantidade\n" +
		"18/08/2026,4.2,NE-I,A,12.345.678/0001-99,4,34\n" +
		"18/08/2026,4.2,NE-I,A,12.345.678/0001-99,8\n"
	postCarga(t, srv, csv)
	drain(w)

	got, ok := pcp.Get(dia(2026, 8, 18), "4.2", "NE-I", "12.345.678/0001-99")
	if !ok || got.Quantidade != 8 {
		t.Fatalf("got ok=%v %+v, want 8 (4,34 com delimitador vírgula não entra como 4)", ok, got)
	}
}

func TestIngestCargaBOMUTF8(t *testing.T) {
	pcp := memory.NewPcpStore()
	jobs := memory.NewJobStore()
	w := worker.New(jobs, pcp, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	postCarga(t, srv, "\ufeff"+minimalCSV())
	drain(w)

	got, ok := pcp.Get(dia(2026, 8, 18), "4.2", "NE-I", "12.345.678/0001-99")
	if !ok || got.Quantidade != 10 {
		t.Fatalf("got ok=%v %+v", ok, got)
	}
}

func postCarga(t *testing.T, srv http.Handler, csv string) string {
	t.Helper()
	body, contentType := multipartCSV(t, "carga.csv", csv)
	req := httptest.NewRequest(http.MethodPost, "/v1/planejamento", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-API-Key", testAPIKey)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ID == "" {
		t.Fatal("expected non-empty job id")
	}
	return resp.ID
}

func drain(w *worker.Worker) {
	for w.ProcessNext() {
	}
}

func multipartCSV(t *testing.T, filename, content string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := io.Copy(part, strings.NewReader(content)); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	return &buf, mw.FormDataContentType()
}

func minimalCSV() string {
	return "data,fase,regional,fornecedor,cnpj,quantidade\n" +
		"18/08/2026,4.2,NE-I,NUH DIGITAL,12.345.678/0001-99,10\n"
}

func dia(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
