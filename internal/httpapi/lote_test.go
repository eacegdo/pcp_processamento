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
	w := worker.New(jobs, escolas)
	srv := httpapi.NewServer(testAPIKey, jobs, w)

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
	if job.Status != "queued" && job.Status != "running" && job.Status != "success" {
		t.Fatalf("job status = %q, want queued/running/success", job.Status)
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
	w := worker.New(jobs, escolas)
	srv := httpapi.NewServer(testAPIKey, jobs, w)

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

	job, ok := jobs.Get(resp.ID)
	if !ok {
		t.Fatalf("job %q not found", resp.ID)
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
			w := worker.New(jobs, escolas)
			srv := httpapi.NewServer(testAPIKey, jobs, w)

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
	w := worker.New(jobs, escolas)
	srv := httpapi.NewServer(testAPIKey, jobs, w)

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
	w := worker.New(jobs, escolas)
	srv := httpapi.NewServer(testAPIKey, jobs, w)

	csv := "inep,oce_tipo_acesso,oce_status_final,oce_pendencia\n" +
		"99999999,remoto,ativo,nenhuma\n" +
		"12345678,presencial,ativo,ok\n"
	body, contentType := multipartCSV(t, "lote.csv", csv)
	req := httptest.NewRequest(http.MethodPost, "/v1/lotes", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-API-Key", testAPIKey)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	if _, ok := escolas.Get("99999999"); ok {
		t.Fatal("INEP inexistente não deve criar Escola")
	}
	got, _ := escolas.Get("12345678")
	want := memory.SituacaoOCE{TipoAcesso: "presencial", Status: "ativo", Pendencia: "ok"}
	if got != want {
		t.Fatalf("situacao = %+v, want %+v", got, want)
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	job, ok := jobs.Get(resp.ID)
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
	w := worker.New(jobs, escolas)
	srv := httpapi.NewServer(testAPIKey, jobs, w)

	csv := "inep,oce_tipo_acesso,oce_status_final,oce_pendencia\n" +
		"12345678,presencial,rascunho,pendente\n" +
		"12345678,remoto,ativo,nenhuma\n"
	body, contentType := multipartCSV(t, "lote.csv", csv)
	req := httptest.NewRequest(http.MethodPost, "/v1/lotes", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-API-Key", testAPIKey)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	got, _ := escolas.Get("12345678")
	want := memory.SituacaoOCE{TipoAcesso: "remoto", Status: "ativo", Pendencia: "nenhuma"}
	if got != want {
		t.Fatalf("situacao = %+v, want última ocorrência %+v", got, want)
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	job, ok := jobs.Get(resp.ID)
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
	w := worker.New(jobs, escolas)
	srv := httpapi.NewServer(testAPIKey, jobs, w)

	csv := "inep,oce_tipo_acesso,oce_status_final,oce_pendencia\n" +
		"12345678,presencial,ativo\n" +
		"12345678,remoto,ativo,nenhuma\n"
	body, contentType := multipartCSV(t, "lote.csv", csv)
	req := httptest.NewRequest(http.MethodPost, "/v1/lotes", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-API-Key", testAPIKey)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

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
	w := worker.New(jobs, escolas)
	srv := httpapi.NewServer(testAPIKey, jobs, w)

	csv := "inep,oce_tipo_acesso,oce_status_final,oce_pendencia\n" +
		"12345678,presencial,,nenhuma\n" +
		"87654321,remoto,ativo,ok\n"
	body, contentType := multipartCSV(t, "lote.csv", csv)
	req := httptest.NewRequest(http.MethodPost, "/v1/lotes", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-API-Key", testAPIKey)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

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

	var resp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	job, ok := jobs.Get(resp.ID)
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
	w := worker.New(jobs, escolas)
	srv := httpapi.NewServer(testAPIKey, jobs, w)

	csv := "inep;oce_tipo_acesso;oce_status_final;oce_pendencia\n" +
		"12345678;presencial;ativo;nenhuma\n"
	body, contentType := multipartCSV(t, "lote.csv", csv)
	req := httptest.NewRequest(http.MethodPost, "/v1/lotes", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-API-Key", testAPIKey)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	got, ok := escolas.Get("12345678")
	if !ok {
		t.Fatal("escola should still exist")
	}
	want := memory.SituacaoOCE{TipoAcesso: "presencial", Status: "ativo", Pendencia: "nenhuma"}
	if got != want {
		t.Fatalf("situacao = %+v, want %+v", got, want)
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
