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

func TestEscolaStoreApplyBatchChamaRpcComLoteInteiro(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotBody map[string]any
	var gotAuth string
	var calls int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	store := supabase.NewEscolaStore(srv.URL, "service-role", srv.Client())
	err := store.ApplyBatch([]domain.ItemLote{
		{
			INEP: "11111111",
			Situacao: domain.SituacaoOCE{
				TipoAcesso: "presencial",
				Status:     "ativo",
				Pendencia:  "nenhuma",
			},
		},
		{
			INEP: "22222222",
			Situacao: domain.SituacaoOCE{
				TipoAcesso: "remoto",
				Status:     "rascunho",
				Pendencia:  "pendente",
			},
		},
	})
	if err != nil {
		t.Fatalf("ApplyBatch: %v", err)
	}

	if calls != 1 {
		t.Fatalf("http calls = %d, want 1 (batch RPC)", calls)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if !strings.HasSuffix(gotPath, "/rpc/aplicar_situacao_oce_lote") {
		t.Fatalf("path = %q, want .../rpc/aplicar_situacao_oce_lote", gotPath)
	}
	if gotAuth != "Bearer service-role" {
		t.Fatalf("Authorization = %q", gotAuth)
	}

	itens, ok := gotBody["itens"].([]any)
	if !ok || len(itens) != 2 {
		t.Fatalf("body = %v, want itens with 2 rows", gotBody)
	}
	first, _ := itens[0].(map[string]any)
	if first["inep"] != "11111111" || first["oce_tipo_acesso"] != "presencial" ||
		first["oce_status"] != "ativo" || first["oce_pendencia"] != "nenhuma" {
		t.Fatalf("first item = %v", first)
	}
	if len(first) != 4 {
		t.Fatalf("item keys = %v, want exactly inep + 3 OCE columns", first)
	}
	second, _ := itens[1].(map[string]any)
	if second["inep"] != "22222222" || second["oce_status"] != "rascunho" {
		t.Fatalf("second item = %v", second)
	}
}

func TestEscolaStoreApplyBatchVazioNaoChamaSupabase(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	store := supabase.NewEscolaStore(srv.URL, "service-role", srv.Client())
	if err := store.ApplyBatch(nil); err != nil {
		t.Fatalf("ApplyBatch: %v", err)
	}
	if calls != 0 {
		t.Fatalf("calls = %d, want 0", calls)
	}
}

func TestEscolaStoreApplyBatchNaoUsaUpsert(t *testing.T) {
	var gotPrefer string
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPrefer = r.Header.Get("Prefer")
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	store := supabase.NewEscolaStore(srv.URL, "service-role", srv.Client())
	_ = store.ApplyBatch([]domain.ItemLote{{
		INEP:     "12345678",
		Situacao: domain.SituacaoOCE{TipoAcesso: "a", Status: "b", Pendencia: "c"},
	}})

	if !strings.Contains(gotPath, "/rpc/") {
		t.Fatalf("path = %q, want RPC update (not table upsert)", gotPath)
	}
	if strings.Contains(strings.ToLower(gotPrefer), "resolution=") {
		t.Fatalf("Prefer must not request upsert resolution, got %q", gotPrefer)
	}
}
