package programado_test

import (
	"strings"
	"testing"
	"time"

	"github.com/wellington/pcp_processamento/internal/domain"
	"github.com/wellington/pcp_processamento/internal/programado"
)

func TestParseJSONUltimaChaveVenceEIgnoraInvalidos(t *testing.T) {
	raw := `[
		{"data":"18/08/2026","fase":"4.2","regional":"NE-I","inep":"1","fornecedor_nome":"A"},
		{"data":"18/08/2026","fase":"4.2","regional":"NE-I","inep":"1","fornecedor_nome":"B","quantidade":2},
		{"data":"18/08/2026","fase":"4.2","regional":"NE-I"},
		{"fase":"4.2","regional":"NE-I","inep":"2"}
	]`
	got, err := programado.ParseJSON(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].INEP != "1" || got[0].FornecedorNome != "B" || got[0].Quantidade != 2 {
		t.Fatalf("got %+v", got[0])
	}
	if got[0].Tipo != domain.TipoProgramado {
		t.Fatalf("tipo = %q", got[0].Tipo)
	}
}

func TestParseJSONEnvelopeItensEINEPNumero(t *testing.T) {
	raw := `{"itens":[{"data":"2026-08-18","fase":"4.2","regional":"NO","inep":29443709}]}`
	got, err := programado.ParseJSON(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if len(got) != 1 || got[0].INEP != "29443709" || got[0].Quantidade != 1 {
		t.Fatalf("got %+v", got)
	}
	if got[0].Data != time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("data = %v", got[0].Data)
	}
	if got[0].RegionalNome != "Norte" {
		t.Fatalf("regional_nome = %q", got[0].RegionalNome)
	}
}

func TestParseJSONAceitaRegionalPorSiglaOuNome(t *testing.T) {
	cases := []struct {
		in, sigla, nome string
	}{
		{"NO", "NO", "Norte"},
		{"norte", "NO", "Norte"},
		{"Norte", "NO", "Norte"},
		{"NE-I", "NE-I", "Nordeste I"},
		{"Nordeste I", "NE-I", "Nordeste I"},
		{"NE-II", "NE-II", "Nordeste II"},
		{"Nordeste II", "NE-II", "Nordeste II"},
		{"SUSE", "SUSE", "Sudeste/Centro-Sul"},
		{"Sudeste/Centro-Sul", "SUSE", "Sudeste/Centro-Sul"},
		{"COSE", "COSE", "Centro-Oeste/Minas"},
		{"Centro-Oeste/Minas", "COSE", "Centro-Oeste/Minas"},
		{"NEI", "NEI", ""},
	}
	for _, tc := range cases {
		raw := `[{"data":"18/08/2026","fase":"4.2","regional":"` + tc.in + `","inep":"1"}]`
		got, err := programado.ParseJSON(strings.NewReader(raw))
		if err != nil {
			t.Fatalf("regional %q: %v", tc.in, err)
		}
		if got[0].Regional != tc.sigla || got[0].RegionalNome != tc.nome {
			t.Fatalf("regional %q -> sigla=%q nome=%q, want %q %q",
				tc.in, got[0].Regional, got[0].RegionalNome, tc.sigla, tc.nome)
		}
	}
}

func TestParseJSONDescartaItemDeOutroMes(t *testing.T) {
	raw := `[
		{"data":"18/08/2026","fase":"4.2","regional":"NE-I","inep":"2"},
		{"data":"01/09/2026","fase":"4.2","regional":"NE-I","inep":"8"}
	]`
	got, err := programado.ParseJSON(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if len(got) != 1 || got[0].INEP != "2" {
		t.Fatalf("got %+v, want só agosto inep 2", got)
	}
}

func TestParseJSONVazioOuSemValidos(t *testing.T) {
	if _, err := programado.ParseJSON(strings.NewReader(`{}`)); err == nil {
		t.Fatal("expected error for empty envelope")
	}
	if _, err := programado.ParseJSON(strings.NewReader(`[]`)); err == nil {
		t.Fatal("expected error for empty array")
	}
	if _, err := programado.ParseJSON(strings.NewReader(`[{"inep":"1"}]`)); err == nil {
		t.Fatal("expected error when no valid objects")
	}
}
