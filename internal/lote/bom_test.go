package lote_test

import (
	"strings"
	"testing"

	"github.com/wellington/pcp_processamento/internal/lote"
)

func TestParseCSVAceitaHeaderComBOMUTF8(t *testing.T) {
	csv := "\ufeffquantidade,cnpj,regional,fase,data,fornecedor\n" +
		"10,12.345.678/0001-99,NE-I,4.2,18/08/2026,NUH\n"
	items, err := lote.ParseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("ParseCSV com BOM: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len = %d, want 1", len(items))
	}
	if items[0].Quantidade != 10 || items[0].FornecedorCNPJ != "12.345.678/0001-99" {
		t.Fatalf("item = %+v", items[0])
	}
	if items[0].RegionalNome != "Nordeste I" {
		t.Fatalf("regional_nome = %q", items[0].RegionalNome)
	}
}

func TestParseCSVSemLinhasValidasERejeitado(t *testing.T) {
	csv := "data,fase,regional,fornecedor,cnpj,quantidade\n" +
		"18/08/2026,4.2,NE-I,A,12.345.678/0001-99,4.34\n"
	_, err := lote.ParseCSV(strings.NewReader(csv))
	if err == nil {
		t.Fatal("expected error for csv sem linhas válidas")
	}
}

func TestParseCSVAceitaCNPJVazioIdentificandoPeloNome(t *testing.T) {
	csv := "data,fase,regional,fornecedor,cnpj,quantidade\n" +
		"01/07/2026,5,NE-I,ARAUJO E ALMEIDA,,4\n" +
		"01/07/2026,5,NE-I,NMA SERVIÇOS,,7\n"
	items, err := lote.ParseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2", len(items))
	}
	if items[0].FornecedorNome != "ARAUJO E ALMEIDA" || items[0].FornecedorCNPJ != "" || items[0].Quantidade != 4 {
		t.Fatalf("item0 = %+v", items[0])
	}
	if items[1].FornecedorNome != "NMA SERVIÇOS" || items[1].FornecedorCNPJ != "" || items[1].Quantidade != 7 {
		t.Fatalf("item1 = %+v", items[1])
	}
}

func TestParseCSVAceitaSemColunaCNPJ(t *testing.T) {
	csv := "data,fase,regional,fornecedor,quantidade\n" +
		"01/07/2026,5,NE-I,ARAUJO E ALMEIDA,4\n"
	items, err := lote.ParseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(items) != 1 || items[0].FornecedorCNPJ != "" || items[0].FornecedorNome != "ARAUJO E ALMEIDA" {
		t.Fatalf("item = %+v", items[0])
	}
}

func TestParseCSVSemCNPJESemNomeEIgnorada(t *testing.T) {
	csv := "data,fase,regional,fornecedor,cnpj,quantidade\n" +
		"01/07/2026,5,NE-I,,,4\n"
	_, err := lote.ParseCSV(strings.NewReader(csv))
	if err == nil {
		t.Fatal("expected error for csv sem linhas válidas")
	}
}

func TestParseCSVUltimaOcorrenciaVenceSemCNPJ(t *testing.T) {
	csv := "data,fase,regional,fornecedor,cnpj,quantidade\n" +
		"01/07/2026,5,NE-I,ARAUJO E ALMEIDA,,4\n" +
		"01/07/2026,5,NE-I,ARAUJO E ALMEIDA,,9\n"
	items, err := lote.ParseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(items) != 1 || items[0].Quantidade != 9 {
		t.Fatalf("items = %+v", items)
	}
}
