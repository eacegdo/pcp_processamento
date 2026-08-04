package supabase_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/wellington/oce_processamento/internal/domain"
	"github.com/wellington/oce_processamento/internal/supabase"
)

func TestJobStoreCreatePersisteOceJobEMantemItensLocais(t *testing.T) {
	var mu sync.Mutex
	var posts []map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/oce_job") {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		defer r.Body.Close()
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		posts = append(posts, body)
		mu.Unlock()

		if r.Header.Get("Prefer") != "return=representation" {
			t.Fatalf("Prefer = %q, want return=representation", r.Header.Get("Prefer"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`[{"id":"11111111-1111-1111-1111-111111111111","status":"queued","total":1,"processadas":0,"file_name":"lote.csv"}]`))
	}))
	defer srv.Close()

	items := []domain.ItemLote{{
		INEP:     "12345678",
		Situacao: domain.SituacaoOCE{TipoAcesso: "presencial", Status: "ativo", Pendencia: "ok"},
	}}
	store := supabase.NewJobStore(srv.URL, "service-role", srv.Client())
	job, err := store.Create(1, "lote.csv", items)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if job.ID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("id = %q", job.ID)
	}
	if job.Status != "queued" || job.Total != 1 || job.FileName != "lote.csv" {
		t.Fatalf("job = %+v", job)
	}
	if len(job.Items) != 1 || job.Items[0].INEP != "12345678" {
		t.Fatalf("items must stay available locally: %+v", job.Items)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(posts) != 1 {
		t.Fatalf("posts = %d, want 1", len(posts))
	}
	body := posts[0]
	// Only Bubble-facing columns — never send items/payload.
	allowed := map[string]bool{
		"status": true, "total": true, "processadas": true, "file_name": true,
	}
	for k := range body {
		if !allowed[k] {
			t.Fatalf("unexpected column %q in oce_job insert: %v", k, body)
		}
	}
	if body["status"] != "queued" || body["total"] != float64(1) || body["processadas"] != float64(0) || body["file_name"] != "lote.csv" {
		t.Fatalf("insert body = %v", body)
	}
}

func TestJobStoreClaimNextEProgressoAtualizamOceJob(t *testing.T) {
	var mu sync.Mutex
	var patches []struct {
		query string
		body  map[string]any
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/oce_job"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`[{"id":"22222222-2222-2222-2222-222222222222","status":"queued","total":2,"processadas":0,"file_name":"a.csv"}]`))
		case r.Method == http.MethodPatch && strings.HasSuffix(r.URL.Path, "/oce_job"):
			var body map[string]any
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &body)
			mu.Lock()
			patches = append(patches, struct {
				query string
				body  map[string]any
			}{query: r.URL.RawQuery, body: body})
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	store := supabase.NewJobStore(srv.URL, "service-role", srv.Client())
	items := []domain.ItemLote{
		{INEP: "1", Situacao: domain.SituacaoOCE{TipoAcesso: "a", Status: "b", Pendencia: "c"}},
		{INEP: "2", Situacao: domain.SituacaoOCE{TipoAcesso: "a", Status: "b", Pendencia: "c"}},
	}
	job, err := store.Create(2, "a.csv", items)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	claimed, ok := store.ClaimNext()
	if !ok || claimed.ID != job.ID || claimed.Status != "running" {
		t.Fatalf("ClaimNext = %+v ok=%v", claimed, ok)
	}
	if err := store.MarkProgress(job.ID, 1); err != nil {
		t.Fatalf("MarkProgress: %v", err)
	}
	if err := store.MarkSuccess(job.ID); err != nil {
		t.Fatalf("MarkSuccess: %v", err)
	}

	got, ok := store.Get(job.ID)
	if !ok || got.Status != "success" || got.Processadas != 2 || got.Restantes != 0 {
		t.Fatalf("Get after success = %+v ok=%v", got, ok)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(patches) != 3 {
		t.Fatalf("patches = %d, want 3 (running, progress, success)", len(patches))
	}
	if patches[0].body["status"] != "running" {
		t.Fatalf("first patch = %v, want status running", patches[0].body)
	}
	if patches[1].body["processadas"] != float64(1) {
		t.Fatalf("progress patch = %v", patches[1].body)
	}
	if patches[2].body["status"] != "success" || patches[2].body["processadas"] != float64(2) {
		t.Fatalf("success patch = %v", patches[2].body)
	}
	for i, p := range patches {
		if p.query != "id=eq.22222222-2222-2222-2222-222222222222" {
			t.Fatalf("patch[%d] query = %q", i, p.query)
		}
		for k := range p.body {
			switch k {
			case "status", "processadas", "error_message":
			default:
				t.Fatalf("patch[%d] unexpected column %q: %v", i, k, p.body)
			}
		}
	}
}

func TestJobStoreMarkFailedPreservaProgressoParcial(t *testing.T) {
	var lastBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`[{"id":"33333333-3333-3333-3333-333333333333","status":"queued","total":3,"processadas":0}]`))
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&lastBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	store := supabase.NewJobStore(srv.URL, "service-role", srv.Client())
	job, _ := store.Create(3, "", []domain.ItemLote{
		{INEP: "1", Situacao: domain.SituacaoOCE{TipoAcesso: "a", Status: "b", Pendencia: "c"}},
		{INEP: "2", Situacao: domain.SituacaoOCE{TipoAcesso: "a", Status: "b", Pendencia: "c"}},
		{INEP: "3", Situacao: domain.SituacaoOCE{TipoAcesso: "a", Status: "b", Pendencia: "c"}},
	})
	_, _ = store.ClaimNext()
	_ = store.MarkProgress(job.ID, 1)
	if err := store.MarkFailed(job.ID, "falha transitória no batch"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	got, ok := store.Get(job.ID)
	if !ok || got.Status != "failed" || got.Processadas != 1 || got.ErrorMessage != "falha transitória no batch" {
		t.Fatalf("job = %+v", got)
	}
	if lastBody["status"] != "failed" || lastBody["error_message"] != "falha transitória no batch" {
		t.Fatalf("last patch = %v", lastBody)
	}
}
