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

	"github.com/wellington/oce_processamento/internal/httpapi"
	"github.com/wellington/oce_processamento/internal/memory"
	"github.com/wellington/oce_processamento/internal/worker"
)

const testAPIKey = "test-api-key"

func TestIngestLoteAutenticadoRetornaIDDoJob(t *testing.T) {
	escolas := memory.NewEscolaStore()
	escolas.Seed("12345678", memory.SituacaoOCE{
		TipoAcesso: "antigo",
		Status:     "antigo",
		Pendencia:  "antiga",
	})
	jobs := memory.NewJobStore()
	srv := httpapi.NewServer(testAPIKey, jobs)

	body, contentType := multipartCSV(t, "lote.csv", minimalCSV())
	req := httptest.NewRequest(http.MethodPost, "/v1/lotes", body)
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

func TestJobAvancaProcessadasPorBatchAteSuccess(t *testing.T) {
	escolas := memory.NewEscolaStore()
	for _, inep := range []string{"11111111", "22222222", "33333333"} {
		escolas.Seed(inep, memory.SituacaoOCE{TipoAcesso: "a", Status: "b", Pendencia: "c"})
	}
	jobs := memory.NewJobStore()
	w := worker.New(jobs, escolas, worker.Config{BatchSize: 2, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	csv := "inep,oce_tipo_acesso,oce_status_final,oce_pendencia\n" +
		"11111111,presencial,ativo,ok\n" +
		"22222222,remoto,ativo,ok\n" +
		"33333333,hibrido,ativo,ok\n"
	id := postLote(t, srv, csv)

	job, ok := jobs.Get(id)
	if !ok {
		t.Fatal("job not found")
	}
	if job.Status != "queued" {
		t.Fatalf("after upload status = %q, want queued", job.Status)
	}

	if !w.ProcessNext() {
		t.Fatal("expected first batch to process")
	}
	job, _ = jobs.Get(id)
	if job.Status != "running" {
		t.Fatalf("after first batch status = %q, want running", job.Status)
	}
	if job.Processadas != 2 {
		t.Fatalf("processadas = %d, want 2", job.Processadas)
	}
	if job.Restantes != 1 {
		t.Fatalf("restantes = %d, want 1", job.Restantes)
	}

	if !w.ProcessNext() {
		t.Fatal("expected second batch to process")
	}
	job, _ = jobs.Get(id)
	if job.Status != "success" {
		t.Fatalf("status = %q, want success", job.Status)
	}
	if job.Processadas != 3 || job.Restantes != 0 {
		t.Fatalf("progress processadas=%d restantes=%d, want 3/0", job.Processadas, job.Restantes)
	}
}

func TestSegundoUploadFicaQueuedEnquantoPrimeiroRunning(t *testing.T) {
	escolas := memory.NewEscolaStore()
	for _, inep := range []string{"11111111", "22222222", "33333333", "44444444"} {
		escolas.Seed(inep, memory.SituacaoOCE{TipoAcesso: "a", Status: "b", Pendencia: "c"})
	}
	jobs := memory.NewJobStore()
	w := worker.New(jobs, escolas, worker.Config{BatchSize: 2, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	csv1 := "inep,oce_tipo_acesso,oce_status_final,oce_pendencia\n" +
		"11111111,presencial,ativo,ok\n" +
		"22222222,remoto,ativo,ok\n" +
		"33333333,hibrido,ativo,ok\n"
	id1 := postLote(t, srv, csv1)
	if !w.ProcessNext() {
		t.Fatal("expected first job to start")
	}
	job1, _ := jobs.Get(id1)
	if job1.Status != "running" {
		t.Fatalf("job1 status = %q, want running", job1.Status)
	}

	csv2 := "inep,oce_tipo_acesso,oce_status_final,oce_pendencia\n" +
		"44444444,presencial,ativo,ok\n"
	id2 := postLote(t, srv, csv2)
	job2, _ := jobs.Get(id2)
	if job2.Status != "queued" {
		t.Fatalf("job2 status = %q, want queued while job1 running", job2.Status)
	}

	drain(w)
	job1, _ = jobs.Get(id1)
	job2, _ = jobs.Get(id2)
	if job1.Status != "success" {
		t.Fatalf("job1 status = %q, want success", job1.Status)
	}
	if job2.Status != "success" {
		t.Fatalf("job2 status = %q, want success after drain", job2.Status)
	}
}

func TestFilaFIFOProcessaNaOrdemDeEnqueue(t *testing.T) {
	escolas := memory.NewEscolaStore()
	escolas.Seed("12345678", memory.SituacaoOCE{TipoAcesso: "a", Status: "b", Pendencia: "c"})
	jobs := memory.NewJobStore()
	w := worker.New(jobs, escolas, worker.Config{BatchSize: 1, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	csvFirst := "inep,oce_tipo_acesso,oce_status_final,oce_pendencia\n" +
		"12345678,primeiro,ativo,ok\n"
	csvSecond := "inep,oce_tipo_acesso,oce_status_final,oce_pendencia\n" +
		"12345678,segundo,ativo,ok\n"
	id1 := postLote(t, srv, csvFirst)
	id2 := postLote(t, srv, csvSecond)

	if !w.ProcessNext() {
		t.Fatal("expected first queued job")
	}
	job1, _ := jobs.Get(id1)
	job2, _ := jobs.Get(id2)
	if job1.Status != "success" {
		t.Fatalf("job1 status = %q, want success (FIFO first)", job1.Status)
	}
	if job2.Status != "queued" {
		t.Fatalf("job2 status = %q, want still queued", job2.Status)
	}
	got, _ := escolas.Get("12345678")
	if got.TipoAcesso != "primeiro" {
		t.Fatalf("situacao after first = %+v, want TipoAcesso=primeiro", got)
	}

	if !w.ProcessNext() {
		t.Fatal("expected second queued job")
	}
	job2, _ = jobs.Get(id2)
	if job2.Status != "success" {
		t.Fatalf("job2 status = %q, want success", job2.Status)
	}
	got, _ = escolas.Get("12345678")
	if got.TipoAcesso != "segundo" {
		t.Fatalf("situacao after second = %+v, want TipoAcesso=segundo", got)
	}
}

func TestFalhaTransitoriaDeBatchERetentada(t *testing.T) {
	escolas := memory.NewEscolaStore()
	escolas.Seed("12345678", memory.SituacaoOCE{TipoAcesso: "a", Status: "b", Pendencia: "c"})
	escolas.FailNext(2) // duas falhas, terceira tentativa sucede
	jobs := memory.NewJobStore()
	w := worker.New(jobs, escolas, worker.Config{BatchSize: 1, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	id := postLote(t, srv, minimalCSV())
	drain(w)

	job, _ := jobs.Get(id)
	if job.Status != "success" {
		t.Fatalf("status = %q error=%q, want success after retries", job.Status, job.ErrorMessage)
	}
	got, _ := escolas.Get("12345678")
	want := memory.SituacaoOCE{TipoAcesso: "presencial", Status: "ativo", Pendencia: "nenhuma"}
	if got != want {
		t.Fatalf("situacao = %+v, want %+v", got, want)
	}
}

func TestAposJobFailedProximoQueuedInicia(t *testing.T) {
	escolas := memory.NewEscolaStore()
	escolas.Seed("11111111", memory.SituacaoOCE{TipoAcesso: "a", Status: "b", Pendencia: "c"})
	escolas.Seed("22222222", memory.SituacaoOCE{TipoAcesso: "a", Status: "b", Pendencia: "c"})
	escolas.FailNext(2)
	jobs := memory.NewJobStore()
	w := worker.New(jobs, escolas, worker.Config{BatchSize: 1, MaxRetries: 2})
	srv := httpapi.NewServer(testAPIKey, jobs)

	csvFail := "inep,oce_tipo_acesso,oce_status_final,oce_pendencia\n" +
		"11111111,presencial,ativo,ok\n"
	csvOk := "inep,oce_tipo_acesso,oce_status_final,oce_pendencia\n" +
		"22222222,remoto,ativo,ok\n"
	idFail := postLote(t, srv, csvFail)
	idOk := postLote(t, srv, csvOk)

	if !w.ProcessNext() {
		t.Fatal("expected failing job to run")
	}
	jobFail, _ := jobs.Get(idFail)
	if jobFail.Status != "failed" {
		t.Fatalf("jobFail status = %q, want failed", jobFail.Status)
	}
	jobOk, _ := jobs.Get(idOk)
	if jobOk.Status != "queued" {
		t.Fatalf("jobOk status = %q, want queued until fail clears running", jobOk.Status)
	}

	if !w.ProcessNext() {
		t.Fatal("expected next queued job after failed")
	}
	jobOk, _ = jobs.Get(idOk)
	if jobOk.Status != "success" {
		t.Fatalf("jobOk status = %q, want success", jobOk.Status)
	}
	got, _ := escolas.Get("22222222")
	if got.TipoAcesso != "remoto" {
		t.Fatalf("segunda aplicação = %+v, want TipoAcesso=remoto", got)
	}
}

func TestFalhaAposRetriesMarcaFailedEPreservaAplicado(t *testing.T) {
	escolas := memory.NewEscolaStore()
	escolas.Seed("11111111", memory.SituacaoOCE{TipoAcesso: "a", Status: "b", Pendencia: "c"})
	escolas.Seed("22222222", memory.SituacaoOCE{TipoAcesso: "a", Status: "b", Pendencia: "c"})
	jobs := memory.NewJobStore()
	w := worker.New(jobs, escolas, worker.Config{BatchSize: 1, MaxRetries: 2})
	srv := httpapi.NewServer(testAPIKey, jobs)

	csv := "inep,oce_tipo_acesso,oce_status_final,oce_pendencia\n" +
		"11111111,presencial,ativo,ok\n" +
		"22222222,remoto,ativo,ok\n"
	id := postLote(t, srv, csv)

	if !w.ProcessNext() {
		t.Fatal("expected first batch")
	}
	job, _ := jobs.Get(id)
	if job.Status != "running" || job.Processadas != 1 {
		t.Fatalf("after first batch status=%q processadas=%d, want running/1", job.Status, job.Processadas)
	}
	got1, _ := escolas.Get("11111111")
	if got1.TipoAcesso != "presencial" {
		t.Fatalf("first escola not applied: %+v", got1)
	}

	escolas.FailNext(2) // esgota MaxRetries=2
	if !w.ProcessNext() {
		t.Fatal("expected failing batch attempt")
	}
	job, _ = jobs.Get(id)
	if job.Status != "failed" {
		t.Fatalf("status = %q, want failed", job.Status)
	}
	if job.ErrorMessage == "" {
		t.Fatal("expected error_message on failed job")
	}
	if job.Processadas != 1 {
		t.Fatalf("processadas = %d, want 1 (parcial)", job.Processadas)
	}

	got1, _ = escolas.Get("11111111")
	want1 := memory.SituacaoOCE{TipoAcesso: "presencial", Status: "ativo", Pendencia: "ok"}
	if got1 != want1 {
		t.Fatalf("update já aplicado perdido: %+v, want %+v", got1, want1)
	}
	got2, _ := escolas.Get("22222222")
	if got2.TipoAcesso != "a" {
		t.Fatalf("segunda escola não deveria ter sido aplicada: %+v", got2)
	}
}

func TestIngestLoteAplicaSituacaoOCEEJobSuccess(t *testing.T) {
	escolas := memory.NewEscolaStore()
	escolas.Seed("12345678", memory.SituacaoOCE{
		TipoAcesso: "antigo",
		Status:     "antigo",
		Pendencia:  "antiga",
	})
	jobs := memory.NewJobStore()
	w := worker.New(jobs, escolas, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	id := postLote(t, srv, minimalCSV())
	drain(w)

	got, ok := escolas.Get("12345678")
	if !ok {
		t.Fatal("escola should still exist")
	}
	want := memory.SituacaoOCE{
		TipoAcesso: "presencial",
		Status:     "ativo",
		Pendencia:  "nenhuma",
	}
	if got != want {
		t.Fatalf("situacao = %+v, want %+v", got, want)
	}

	job, ok := jobs.Get(id)
	if !ok {
		t.Fatalf("job %q not found", id)
	}
	if job.Status != "success" {
		t.Fatalf("job status = %q, want success", job.Status)
	}
	if job.Total != 1 || job.Processadas != 1 {
		t.Fatalf("progress total=%d processadas=%d, want 1/1", job.Total, job.Processadas)
	}
}

func TestIngestLoteSemAPIKeyValidaERejeitado(t *testing.T) {
	cases := []struct {
		name   string
		apiKey string
	}{
		{name: "ausente", apiKey: ""},
		{name: "errada", apiKey: "chave-errada"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			escolas := memory.NewEscolaStore()
			escolas.Seed("12345678", memory.SituacaoOCE{TipoAcesso: "a", Status: "b", Pendencia: "c"})
			jobs := memory.NewJobStore()
			srv := httpapi.NewServer(testAPIKey, jobs)

			body, contentType := multipartCSV(t, "lote.csv", minimalCSV())
			req := httptest.NewRequest(http.MethodPost, "/v1/lotes", body)
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
			got, _ := escolas.Get("12345678")
			if got.Status != "b" {
				t.Fatalf("escola was mutated: %+v", got)
			}
		})
	}
}

func TestIngestLoteCSVInvalidoERejeitado(t *testing.T) {
	escolas := memory.NewEscolaStore()
	escolas.Seed("12345678", memory.SituacaoOCE{TipoAcesso: "a", Status: "b", Pendencia: "c"})
	jobs := memory.NewJobStore()
	srv := httpapi.NewServer(testAPIKey, jobs)

	body, contentType := multipartCSV(t, "lote.csv", "foo,bar\n1,2\n")
	req := httptest.NewRequest(http.MethodPost, "/v1/lotes", body)
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
	got, _ := escolas.Get("12345678")
	if got.Status != "b" {
		t.Fatalf("escola was mutated: %+v", got)
	}
}

func TestIngestLoteINEPInexistenteNaoCriaEscolaNemFalhaJob(t *testing.T) {
	escolas := memory.NewEscolaStore()
	escolas.Seed("12345678", memory.SituacaoOCE{TipoAcesso: "antigo", Status: "antigo", Pendencia: "antiga"})
	jobs := memory.NewJobStore()
	w := worker.New(jobs, escolas, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	csv := "inep,oce_tipo_acesso,oce_status_final,oce_pendencia\n" +
		"99999999,remoto,ativo,nenhuma\n" +
		"12345678,presencial,ativo,ok\n"
	id := postLote(t, srv, csv)
	drain(w)

	if _, ok := escolas.Get("99999999"); ok {
		t.Fatal("INEP inexistente não deve criar Escola")
	}
	got, _ := escolas.Get("12345678")
	want := memory.SituacaoOCE{TipoAcesso: "presencial", Status: "ativo", Pendencia: "ok"}
	if got != want {
		t.Fatalf("situacao = %+v, want %+v", got, want)
	}

	job, ok := jobs.Get(id)
	if !ok {
		t.Fatal("job not found")
	}
	if job.Status != "success" {
		t.Fatalf("job status = %q, want success", job.Status)
	}
	if job.Total != 2 || job.Processadas != 2 {
		t.Fatalf("progress total=%d processadas=%d, want 2/2 (inclui INEP inexistente)", job.Total, job.Processadas)
	}
}

func TestIngestLoteDuplicataINEPUltimaOcorrenciaVence(t *testing.T) {
	escolas := memory.NewEscolaStore()
	escolas.Seed("12345678", memory.SituacaoOCE{TipoAcesso: "antigo", Status: "antigo", Pendencia: "antiga"})
	jobs := memory.NewJobStore()
	w := worker.New(jobs, escolas, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	csv := "inep,oce_tipo_acesso,oce_status_final,oce_pendencia\n" +
		"12345678,presencial,rascunho,pendente\n" +
		"12345678,remoto,ativo,nenhuma\n"
	id := postLote(t, srv, csv)
	drain(w)

	got, _ := escolas.Get("12345678")
	want := memory.SituacaoOCE{TipoAcesso: "remoto", Status: "ativo", Pendencia: "nenhuma"}
	if got != want {
		t.Fatalf("situacao = %+v, want última ocorrência %+v", got, want)
	}

	job, ok := jobs.Get(id)
	if !ok {
		t.Fatal("job not found")
	}
	if job.Total != 1 {
		t.Fatalf("total = %d, want 1 após dedupe", job.Total)
	}
	if job.Status != "success" || job.Processadas != 1 {
		t.Fatalf("job status=%q processadas=%d, want success/1", job.Status, job.Processadas)
	}
}

func TestIngestLoteIgnoraLinhaComColunasInsuficientes(t *testing.T) {
	escolas := memory.NewEscolaStore()
	escolas.Seed("12345678", memory.SituacaoOCE{TipoAcesso: "antigo", Status: "antigo", Pendencia: "antiga"})
	jobs := memory.NewJobStore()
	w := worker.New(jobs, escolas, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	csv := "inep,oce_tipo_acesso,oce_status_final,oce_pendencia\n" +
		"12345678,presencial,ativo\n" +
		"12345678,remoto,ativo,nenhuma\n"
	postLote(t, srv, csv)
	drain(w)

	got, _ := escolas.Get("12345678")
	want := memory.SituacaoOCE{TipoAcesso: "remoto", Status: "ativo", Pendencia: "nenhuma"}
	if got != want {
		t.Fatalf("Situação OCE = %+v, want %+v", got, want)
	}
}

func TestIngestLoteIgnoraLinhaComCampoVazio(t *testing.T) {
	escolas := memory.NewEscolaStore()
	escolas.Seed("12345678", memory.SituacaoOCE{TipoAcesso: "preservar", Status: "preservar", Pendencia: "preservar"})
	escolas.Seed("87654321", memory.SituacaoOCE{TipoAcesso: "antigo", Status: "antigo", Pendencia: "antiga"})
	jobs := memory.NewJobStore()
	w := worker.New(jobs, escolas, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	csv := "inep,oce_tipo_acesso,oce_status_final,oce_pendencia\n" +
		"12345678,presencial,,nenhuma\n" +
		"87654321,remoto,ativo,ok\n"
	id := postLote(t, srv, csv)
	drain(w)

	gotPreservada, _ := escolas.Get("12345678")
	wantPreservada := memory.SituacaoOCE{TipoAcesso: "preservar", Status: "preservar", Pendencia: "preservar"}
	if gotPreservada != wantPreservada {
		t.Fatalf("linha incompleta limpou situacao: %+v, want %+v", gotPreservada, wantPreservada)
	}
	gotAplicada, _ := escolas.Get("87654321")
	wantAplicada := memory.SituacaoOCE{TipoAcesso: "remoto", Status: "ativo", Pendencia: "ok"}
	if gotAplicada != wantAplicada {
		t.Fatalf("situacao valida = %+v, want %+v", gotAplicada, wantAplicada)
	}

	job, ok := jobs.Get(id)
	if !ok {
		t.Fatal("job not found")
	}
	if job.Total != 1 {
		t.Fatalf("total = %d, want 1 (só linhas válidas)", job.Total)
	}
}

func TestIngestLoteCSVComPontoEVirgulaAplicaSituacaoOCE(t *testing.T) {
	escolas := memory.NewEscolaStore()
	escolas.Seed("12345678", memory.SituacaoOCE{TipoAcesso: "antigo", Status: "antigo", Pendencia: "antiga"})
	jobs := memory.NewJobStore()
	w := worker.New(jobs, escolas, worker.Config{BatchSize: 200, MaxRetries: 3})
	srv := httpapi.NewServer(testAPIKey, jobs)

	csv := "inep;oce_tipo_acesso;oce_status_final;oce_pendencia\n" +
		"12345678;presencial;ativo;nenhuma\n"
	postLote(t, srv, csv)
	drain(w)

	got, ok := escolas.Get("12345678")
	if !ok {
		t.Fatal("escola should still exist")
	}
	want := memory.SituacaoOCE{TipoAcesso: "presencial", Status: "ativo", Pendencia: "nenhuma"}
	if got != want {
		t.Fatalf("situacao = %+v, want %+v", got, want)
	}
}

func postLote(t *testing.T, srv http.Handler, csv string) string {
	t.Helper()
	body, contentType := multipartCSV(t, "lote.csv", csv)
	req := httptest.NewRequest(http.MethodPost, "/v1/lotes", body)
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
	return "inep,oce_tipo_acesso,oce_status_final,oce_pendencia\n" +
		"12345678,presencial,ativo,nenhuma\n"
}
