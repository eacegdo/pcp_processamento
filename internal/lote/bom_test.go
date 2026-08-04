package lote_test

import (
	"strings"
	"testing"

	"github.com/wellington/oce_processamento/internal/lote"
)

func TestParseCSVAceitaHeaderComBOMUTF8(t *testing.T) {
	csv := "\ufeffoce_tipo_acesso,oce_status_final,oce_pendencia,inep\n" +
		"presencial,ativo,nenhuma,12345678\n"
	items, err := lote.ParseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("ParseCSV com BOM: %v", err)
	}
	if len(items) != 1 || items[0].INEP != "12345678" {
		t.Fatalf("items = %+v", items)
	}
	if items[0].Situacao.Status != "ativo" {
		t.Fatalf("status = %q, want ativo", items[0].Situacao.Status)
	}
}

func TestParseCSVAceitaColunasForaDeOrdem(t *testing.T) {
	csv := "oce_tipo_acesso,oce_status_final,oce_pendencia,inep\n" +
		"remoto,rascunho,pendente,87654321\n"
	items, err := lote.ParseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if items[0].INEP != "87654321" || items[0].Situacao.TipoAcesso != "remoto" {
		t.Fatalf("items = %+v", items)
	}
}
