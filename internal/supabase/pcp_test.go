package supabase_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wellington/pcp_processamento/internal/domain"
	"github.com/wellington/pcp_processamento/internal/supabase"
)

func TestPcpStoreApplyBatchChamaRpcComBatchInteiro(t *testing.T) {
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

	store := supabase.NewPcpStore(srv.URL, "service-role", srv.Client())
	err := store.ApplyBatch([]domain.ItemCarga{
		{
			Data:           time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC),
			Fase:           "4.2",
			Regional:       "NE-I",
			RegionalNome:   "Nordeste I",
			FornecedorNome: "NUH",
			FornecedorCNPJ: "12.345.678/0001-99",
			Quantidade:     10,
		},
		{
			Data:           time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
			Fase:           "4.2",
			Regional:       "NO",
			RegionalNome:   "Norte",
			FornecedorCNPJ: "22.222.222/0001-22",
			Quantidade:     0,
		},
	})
	if err != nil {
		t.Fatalf("ApplyBatch: %v", err)
	}
	if calls != 1 {
		t.Fatalf("http calls = %d, want 1", calls)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q", gotMethod)
	}
	if !strings.HasSuffix(gotPath, "/rpc/aplicar_carga_planejamento") {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "Bearer service-role" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	itens, ok := gotBody["itens"].([]any)
	if !ok || len(itens) != 2 {
		t.Fatalf("body = %v", gotBody)
	}
	first, _ := itens[0].(map[string]any)
	if first["data"] != "2026-08-18" || first["fase"] != "4.2" || first["regional"] != "NE-I" ||
		first["regional_nome"] != "Nordeste I" || first["fornecedor_cnpj"] != "12.345.678/0001-99" ||
		first["quantidade"] != float64(10) {
		t.Fatalf("first item = %v", first)
	}
}

func TestPcpStoreApplyBatchProgramadoChamaRpcAplicarProgramado(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	store := supabase.NewPcpStore(srv.URL, "service-role", srv.Client())
	prov := false
	err := store.ApplyBatch([]domain.ItemCarga{{
		Tipo:           domain.TipoProgramado,
		Data:           time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC),
		Fase:           "4.2",
		Regional:       "NE-I",
		INEP:           "12345678",
		Quantidade:     1,
		Provisoria:     &prov,
		FornecedorCNPJ: "12.345.678/0001-99",
	}})
	if err != nil {
		t.Fatalf("ApplyBatch: %v", err)
	}
	if !strings.HasSuffix(gotPath, "/rpc/aplicar_programado") {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestPcpStoreApplyBatchVazioNaoChamaSupabase(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	store := supabase.NewPcpStore(srv.URL, "service-role", srv.Client())
	if err := store.ApplyBatch(nil); err != nil {
		t.Fatalf("ApplyBatch: %v", err)
	}
	if calls != 0 {
		t.Fatalf("calls = %d, want 0", calls)
	}
}
