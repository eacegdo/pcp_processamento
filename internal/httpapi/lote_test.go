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
