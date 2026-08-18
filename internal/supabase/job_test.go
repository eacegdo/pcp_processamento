package supabase_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wellington/pcp_processamento/internal/domain"
	"github.com/wellington/pcp_processamento/internal/supabase"
)

func sampleItem() domain.ItemCarga {
	return domain.ItemCarga{
		Data:           time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC),
		Fase:           "4.2",
		Regional:       "NE-I",
		RegionalNome:   "Nordeste I",
		FornecedorCNPJ: "12.345.678/0001-99",
		Quantidade:     10,
	}
}

func TestJobStoreCreatePersistePcpJobEMantemItensLocais(t *testing.T) {
	var mu sync.Mutex
	var posts []map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/pcp_job") {
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
		_, _ = w.Write([]byte(`[{"id":"11111111-1111-1111-1111-111111111111","status":"queued","total":1,"processadas":0,"file_name":"carga.csv"}]`))
	}))
	defer srv.Close()

	store := supabase.NewJobStore(srv.URL, "service-role", srv.Client())
	job, err := store.Create(1, domain.TipoPlanejado, "carga.csv", []domain.ItemCarga{sampleItem()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if job.ID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("id = %q", job.ID)
	}
	if job.Status != "queued" || job.Total != 1 || job.FileName != "carga.csv" || job.Tipo != domain.TipoPlanejado {
		t.Fatalf("job = %+v", job)
	}
	if len(job.Items) != 1 || job.Items[0].FornecedorCNPJ != "12.345.678/0001-99" {
		t.Fatalf("items must stay available locally: %+v", job.Items)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(posts) != 1 {
		t.Fatalf("posts = %d, want 1", len(posts))
	}
	body := posts[0]
	allowed := map[string]bool{
		"status": true, "tipo": true, "total": true, "processadas": true, "file_name": true,
	}
	for k := range body {
		if !allowed[k] {
			t.Fatalf("unexpected column %q in pcp_job insert: %v", k, body)
		}
	}
	if body["tipo"] != domain.TipoPlanejado {
		t.Fatalf("tipo = %v, want planejado", body["tipo"])
	}
}

func TestJobStoreClaimNextEProgressoAtualizamPcpJob(t *testing.T) {
	var mu sync.Mutex
	var patches []struct {
		query string
		body  map[string]any
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/pcp_job"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`[{"id":"22222222-2222-2222-2222-222222222222","status":"queued","total":2,"processadas":0,"file_name":"a.csv"}]`))
		case r.Method == http.MethodPatch && strings.HasSuffix(r.URL.Path, "/pcp_job"):
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
	items := []domain.ItemCarga{sampleItem(), sampleItem()}
	job, err := store.Create(2, domain.TipoPlanejado, "a.csv", items)
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
		t.Fatalf("patches = %d, want 3", len(patches))
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
	job, _ := store.Create(3, domain.TipoProgramado, "", []domain.ItemCarga{sampleItem(), sampleItem(), sampleItem()})
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
