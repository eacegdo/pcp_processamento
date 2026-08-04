package supabase_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wellington/oce_processamento/internal/domain"
	"github.com/wellington/oce_processamento/internal/supabase"
)

func TestEscolaStoreApplyBatchAtualizaSoColunasOcePorINEP(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotQuery string
	var gotBody map[string]any
	var gotAuth string
	var gotPrefer string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		gotPrefer = r.Header.Get("Prefer")
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	store := supabase.NewEscolaStore(srv.URL, "service-role", srv.Client())
	err := store.ApplyBatch([]domain.ItemLote{{
		INEP: "12345678",
		Situacao: domain.SituacaoOCE{
			TipoAcesso: "presencial",
			Status:     "ativo",
			Pendencia:  "nenhuma",
		},
	}})
	if err != nil {
		t.Fatalf("ApplyBatch: %v", err)
	}

	if gotMethod != http.MethodPatch {
		t.Fatalf("method = %q, want PATCH", gotMethod)
	}
	if !strings.HasSuffix(gotPath, "/escola") {
		t.Fatalf("path = %q, want .../escola", gotPath)
	}
	if gotQuery != "inep=eq.12345678" {
		t.Fatalf("query = %q, want inep=eq.12345678", gotQuery)
	}
	if gotAuth != "Bearer service-role" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if !strings.Contains(gotPrefer, "return=minimal") {
		t.Fatalf("Prefer = %q, want return=minimal", gotPrefer)
	}
	wantKeys := map[string]string{
		"oce_tipo_acesso": "presencial",
		"oce_status":      "ativo",
		"oce_pendencia":   "nenhuma",
	}
	if len(gotBody) != 3 {
		t.Fatalf("body keys = %v, want exactly 3 OCE columns", gotBody)
	}
	for k, v := range wantKeys {
		if gotBody[k] != v {
			t.Fatalf("body[%q] = %v, want %q", k, gotBody[k], v)
		}
	}
}

func TestEscolaStoreApplyBatchINEPInexistenteENoOp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// PostgREST PATCH with no matching rows still returns success.
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	store := supabase.NewEscolaStore(srv.URL, "service-role", srv.Client())
	err := store.ApplyBatch([]domain.ItemLote{{
		INEP:     "99999999",
		Situacao: domain.SituacaoOCE{TipoAcesso: "a", Status: "b", Pendencia: "c"},
	}})
	if err != nil {
		t.Fatalf("missing INEP must be no-op, got %v", err)
	}
}

func TestEscolaStoreApplyBatchNaoUsaUpsert(t *testing.T) {
	var gotPrefer string
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPrefer = r.Header.Get("Prefer")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	store := supabase.NewEscolaStore(srv.URL, "service-role", srv.Client())
	_ = store.ApplyBatch([]domain.ItemLote{{
		INEP:     "12345678",
		Situacao: domain.SituacaoOCE{TipoAcesso: "a", Status: "b", Pendencia: "c"},
	}})

	if gotMethod != http.MethodPatch {
		t.Fatalf("method = %q, want PATCH (never POST upsert)", gotMethod)
	}
	if strings.Contains(strings.ToLower(gotPrefer), "resolution=") {
		t.Fatalf("Prefer must not request upsert resolution, got %q", gotPrefer)
	}
}
