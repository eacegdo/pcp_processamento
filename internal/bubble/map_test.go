package bubble_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/wellington/pcp_processamento/internal/bubble"
)

func testdata(t *testing.T, name string) []byte {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDecodeFolhaOSP(t *testing.T) {
	var env bubble.Envelope
	if err := json.Unmarshal(testdata(t, "fr_osp_page.json"), &env); err != nil {
		t.Fatal(err)
	}
	var rows []bubble.FolhaOSP
	if err := json.Unmarshal(env.Response.Results, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("len=%d", len(rows))
	}
	f := rows[0]
	if f.INEP != "15026868" || f.UF != "PA" || f.EscolaID == "" || f.OSPID == "" {
		t.Fatalf("folha = %+v", f)
	}
	if f.Status != "Aprovado" {
		t.Fatalf("status = %q", f.Status)
	}
}

func TestJuncaoFolhaEscolaPreencheDimensoesDaEscola(t *testing.T) {
	var env bubble.Envelope
	_ = json.Unmarshal(testdata(t, "fr_osp_page.json"), &env)
	var rows []bubble.FolhaOSP
	_ = json.Unmarshal(env.Response.Results, &rows)
	var esc bubble.Escola
	if err := json.Unmarshal(testdata(t, "escola.json"), &esc); err != nil {
		t.Fatal(err)
	}

	j := bubble.JuncaoFolhaEscola(rows[0], &esc)
	if j.INEP != "15026868" || j.UF != "PA" || j.Fase != "3" {
		t.Fatalf("juncao = %+v", j)
	}
	if j.Regional != "NO" || j.RegionalNome != "Norte" {
		t.Fatalf("regional = %q %q", j.Regional, j.RegionalNome)
	}
	if j.FornecedorNome != "Q13 TECNOLOGIA" || j.FornecedorCNPJ != "30.161.238/0001-60" {
		t.Fatalf("fornecedor = %+v", j)
	}
	if j.Quantidade != 1 {
		t.Fatalf("quantidade = %d", j.Quantidade)
	}
}

func TestProgramadoUsaPrevisaoEntregaDoOSP(t *testing.T) {
	var env bubble.Envelope
	_ = json.Unmarshal(testdata(t, "fr_osp_page.json"), &env)
	var rows []bubble.FolhaOSP
	_ = json.Unmarshal(env.Response.Results, &rows)
	var esc bubble.Escola
	_ = json.Unmarshal(testdata(t, "escola.json"), &esc)
	folha := rows[0]
	folha.ListaContratosInstalacao = []string{"c1"}
	osp := bubble.OSP{
		ID:              folha.OSPID,
		Status:          "Nota Fiscal",
		PrevisaoEntrega: "2026-08-08T17:44:00.000Z",
	}
	contratos := map[string]bubble.ContratoInstalacao{
		"c1": {ID: "c1", Descricao: "Kit Cobertura Wi-Fi", TipoDeObra: bubble.TipoObraRedeInterna},
	}

	item, skip := bubble.ProgramadoDaFolha(osp, folha, &esc, contratos, nil)
	if skip != "" {
		t.Fatalf("skip = %q", skip)
	}
	if item.Data.Format("2006-01-02") != "2026-08-08" {
		t.Fatalf("data = %s", item.Data.Format("2006-01-02"))
	}
	if item.INEP != "15026868" || item.Quantidade != 1 || item.Regional != "NO" {
		t.Fatalf("%+v", item)
	}
	if item.Provisoria == nil || *item.Provisoria != true {
		t.Fatalf("osnum vazio deve ser provisoria=true, got %v", item.Provisoria)
	}

	osp.Status = "Reprovado"
	if _, skip = bubble.ProgramadoDaFolha(osp, folha, &esc, contratos, nil); skip != bubble.SkipOSPReprovada {
		t.Fatalf("reprovado skip = %q", skip)
	}
}

func TestProgramadoUsaDataRelatorioQuandoConectada(t *testing.T) {
	var env bubble.Envelope
	_ = json.Unmarshal(testdata(t, "fr_osp_page.json"), &env)
	var rows []bubble.FolhaOSP
	_ = json.Unmarshal(env.Response.Results, &rows)
	var esc bubble.Escola
	_ = json.Unmarshal(testdata(t, "escola.json"), &esc)
	esc.StatusGeral = "Conectada"
	var imp bubble.ImportacaoEscola
	_ = json.Unmarshal(testdata(t, "importacao_escola.json"), &imp)
	folha := rows[0]
	folha.ListaContratosInstalacao = []string{"c1"}
	n := 13.0
	osp := bubble.OSP{
		ID:              folha.OSPID,
		Status:          "Nota Fiscal",
		PrevisaoEntrega: "2026-08-08T17:44:00.000Z",
		OSnum:           &n,
	}
	contratos := map[string]bubble.ContratoInstalacao{
		"c1": {ID: "c1", Descricao: "Kit Cobertura Wi-Fi", TipoDeObra: bubble.TipoObraRedeInterna},
	}

	item, skip := bubble.ProgramadoDaFolha(osp, folha, &esc, contratos, &imp)
	if skip != "" {
		t.Fatalf("skip = %q", skip)
	}
	if item.Data.Format("2006-01-02") != "2026-08-10" {
		t.Fatalf("data = %s, want data_relatorio", item.Data.Format("2006-01-02"))
	}
	if item.Provisoria == nil || *item.Provisoria {
		t.Fatalf("osnum preenchido deve ser provisoria=false")
	}
}

func TestProgramadoIgnoraDataRelatorioSeNaoConectada(t *testing.T) {
	var env bubble.Envelope
	_ = json.Unmarshal(testdata(t, "fr_osp_page.json"), &env)
	var rows []bubble.FolhaOSP
	_ = json.Unmarshal(env.Response.Results, &rows)
	var esc bubble.Escola
	_ = json.Unmarshal(testdata(t, "escola.json"), &esc)
	esc.StatusGeral = "Em planejamento"
	var imp bubble.ImportacaoEscola
	_ = json.Unmarshal(testdata(t, "importacao_escola.json"), &imp)
	folha := rows[0]
	folha.ListaContratosInstalacao = []string{"c1"}
	osp := bubble.OSP{Status: "Nota Fiscal", PrevisaoEntrega: "2026-08-08T17:44:00.000Z"}
	contratos := map[string]bubble.ContratoInstalacao{
		"c1": {ID: "c1", Descricao: "Kit", TipoDeObra: bubble.TipoObraRedeInterna},
	}
	item, skip := bubble.ProgramadoDaFolha(osp, folha, &esc, contratos, &imp)
	if skip != "" {
		t.Fatal(skip)
	}
	if item.Data.Format("2006-01-02") != "2026-08-08" {
		t.Fatalf("data = %s, want previsão", item.Data.Format("2006-01-02"))
	}
}

func TestContratoKitRedeInterna(t *testing.T) {
	ok := bubble.ContratoInstalacao{Descricao: "kit cobertura", TipoDeObra: bubble.TipoObraRedeInterna}
	if !bubble.ContratoKitRedeInterna(ok) {
		t.Fatal("kit+RI should match")
	}
	if bubble.ContratoKitRedeInterna(bubble.ContratoInstalacao{Descricao: "Taxa instalação", TipoDeObra: bubble.TipoObraRedeInterna}) {
		t.Fatal("sem kit")
	}
	if bubble.ContratoKitRedeInterna(bubble.ContratoInstalacao{Descricao: "Kit X", TipoDeObra: "3-INSTALAÇÃO_DE_ACESSO"}) {
		t.Fatal("obra errada")
	}
}

func TestOSPNoMes(t *testing.T) {
	osp := bubble.OSP{Status: "Concluído", PrevisaoEntrega: "2026-08-08T17:44:00.000Z"}
	ago := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if !bubble.OSPNoMes(osp, ago) || !bubble.OSPNaoReprovada(osp) {
		t.Fatal("agosto")
	}
	if bubble.OSPNoMes(osp, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("julho")
	}
}

func TestMesCivil(t *testing.T) {
	got, err := bubble.MesCivil("2026-08")
	if err != nil {
		t.Fatal(err)
	}
	if got.Year() != 2026 || got.Month() != 8 || got.Day() != 1 {
		t.Fatalf("%v", got)
	}
	if _, err := bubble.MesCivil("agosto"); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodeOSPRegionalAceitaListaOuTexto(t *testing.T) {
	var lista bubble.OSP
	if err := json.Unmarshal([]byte(`{"Regional":["Norte","Sudeste/Centro-Sul"]}`), &lista); err != nil {
		t.Fatal(err)
	}
	if string(lista.Regional) != "Norte" {
		t.Fatalf("lista = %q", lista.Regional)
	}
	var texto bubble.OSP
	if err := json.Unmarshal([]byte(`{"Regional":"NE-I"}`), &texto); err != nil {
		t.Fatal(err)
	}
	if string(texto.Regional) != "NE-I" {
		t.Fatalf("texto = %q", texto.Regional)
	}
}
