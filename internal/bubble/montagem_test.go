package bubble_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wellington/pcp_processamento/internal/bubble"
)

// fonteFalsa é a porta de busca em memória: os mesmos dados que o Bubble
// devolveria, sem HTTP. Os filtros que a Data API faria por constraint
// (mês da previsão, mês da conexão, INEP, IDs) são feitos aqui.
type fonteFalsa struct {
	osps        []bubble.OSP
	folhas      []bubble.FolhaOSP
	escolas     []bubble.Escola
	contratos   []bubble.ContratoInstalacao
	importacoes []bubble.ImportacaoEscola

	erroImportacoesDoMes error
}

func (f *fonteFalsa) OSPsDoMes(mes time.Time) ([]bubble.OSP, error) {
	var out []bubble.OSP
	for _, osp := range f.osps {
		if bubble.OSPNoMes(osp, mes) && bubble.OSPNaoReprovada(osp) {
			out = append(out, osp)
		}
	}
	return out, nil
}

func (f *fonteFalsa) OSPsPorIDs(ids []string) (map[string]bubble.OSP, error) {
	quer := conjunto(ids)
	out := map[string]bubble.OSP{}
	for _, osp := range f.osps {
		if _, ok := quer[osp.ID]; ok {
			out[osp.ID] = osp
		}
	}
	return out, nil
}

func (f *fonteFalsa) ImportacoesDoMes(mes time.Time) ([]bubble.ImportacaoEscola, error) {
	if f.erroImportacoesDoMes != nil {
		return nil, f.erroImportacoesDoMes
	}
	var out []bubble.ImportacaoEscola
	for _, imp := range f.importacoes {
		if d, ok := bubble.DataConexao(&imp); ok && bubble.DataNoMes(d, mes) {
			out = append(out, imp)
		}
	}
	return out, nil
}

func (f *fonteFalsa) FolhasPorOSPs(ospIDs []string) ([]bubble.FolhaOSP, error) {
	quer := conjunto(ospIDs)
	var out []bubble.FolhaOSP
	for _, folha := range f.folhas {
		if _, ok := quer[folha.OSPID]; ok {
			out = append(out, folha)
		}
	}
	return out, nil
}

func (f *fonteFalsa) FolhasPorINEPs(ineps []string) ([]bubble.FolhaOSP, error) {
	quer := conjunto(ineps)
	if len(quer) == 0 {
		return nil, nil
	}
	var out []bubble.FolhaOSP
	for _, folha := range f.folhas {
		if _, ok := quer[strings.TrimSpace(folha.INEP)]; ok {
			out = append(out, folha)
		}
	}
	return out, nil
}

func (f *fonteFalsa) ContratosPorIDs(ids []string) (map[string]bubble.ContratoInstalacao, error) {
	quer := conjunto(ids)
	out := map[string]bubble.ContratoInstalacao{}
	for _, ct := range f.contratos {
		if _, ok := quer[ct.ID]; ok {
			out[ct.ID] = ct
		}
	}
	return out, nil
}

func (f *fonteFalsa) EscolasPorIDs(ids []string) (map[string]bubble.Escola, error) {
	quer := conjunto(ids)
	out := map[string]bubble.Escola{}
	for _, esc := range f.escolas {
		if _, ok := quer[esc.ID]; ok {
			out[esc.ID] = esc
		}
	}
	return out, nil
}

func (f *fonteFalsa) ImportacoesPorINEPs(ineps []string) (map[string]*bubble.ImportacaoEscola, error) {
	quer := conjunto(ineps)
	porINEP := map[string][]bubble.ImportacaoEscola{}
	for _, imp := range f.importacoes {
		inep := strings.TrimSpace(imp.INEP)
		if _, ok := quer[inep]; !ok {
			continue
		}
		porINEP[inep] = append(porINEP[inep], imp)
	}
	out := map[string]*bubble.ImportacaoEscola{}
	for inep, rows := range porINEP {
		out[inep] = bubble.ImportacaoComRelatorio(rows)
	}
	return out, nil
}

func conjunto(vs []string) map[string]struct{} {
	out := make(map[string]struct{}, len(vs))
	for _, v := range vs {
		v = strings.TrimSpace(v)
		if v != "" {
			out[v] = struct{}{}
		}
	}
	return out
}

var (
	agosto = time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	julho  = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
)

func kitRI(id string) bubble.ContratoInstalacao {
	return bubble.ContratoInstalacao{ID: id, Descricao: "Kit Cobertura Wi-Fi", TipoDeObra: bubble.TipoObraRedeInterna}
}

func escolaOK(id, inep, status string) bubble.Escola {
	return bubble.Escola{
		ID: id, INEP: inep, UF: "PA", Fase: "3", Regional: "Norte",
		StatusGeral: status, FornecedorRI: "Q13", CNPJFornecedorRI: "30.161.238/0001-60",
	}
}

func skipDe(t *testing.T, got bubble.Puxado, inep string) bubble.MotivoSkip {
	t.Helper()
	for _, s := range got.Skips {
		if s.INEP == inep {
			return s.Motivo
		}
	}
	t.Fatalf("sem skip para inep %q: %+v", inep, got.Skips)
	return ""
}

func TestMontarMesOSPComPrevisaoNoMesEntra(t *testing.T) {
	f := &fonteFalsa{
		osps:      []bubble.OSP{{ID: "osp1", Status: "Nota Fiscal", PrevisaoEntrega: "2026-08-08T17:44:00.000Z", FRs: []string{"fr1"}}},
		folhas:    []bubble.FolhaOSP{{ID: "fr1", INEP: "1", EscolaID: "e1", OSPID: "osp1", ListaContratosInstalacao: []string{"c1"}}},
		escolas:   []bubble.Escola{escolaOK("e1", "1", "Em planejamento")},
		contratos: []bubble.ContratoInstalacao{kitRI("c1")},
	}
	got, err := bubble.MontarMes(f, agosto)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Itens) != 1 {
		t.Fatalf("itens=%d skips=%+v", len(got.Itens), got.Skips)
	}
	if d := got.Itens[0].Data.Format("2006-01-02"); d != "2026-08-08" {
		t.Fatalf("data = %s", d)
	}
	if got.Itens[0].Fase != "3" || got.Itens[0].Regional != "NO" || got.Itens[0].INEP != "1" {
		t.Fatalf("%+v", got.Itens[0])
	}
}

func TestMontarMesEscolaConectadaGravaDataDaConexao(t *testing.T) {
	f := &fonteFalsa{
		osps:        []bubble.OSP{{ID: "osp1", Status: "Nota Fiscal", PrevisaoEntrega: "2026-08-08T17:44:00.000Z", FRs: []string{"fr1"}}},
		folhas:      []bubble.FolhaOSP{{ID: "fr1", INEP: "1", EscolaID: "e1", OSPID: "osp1", ListaContratosInstalacao: []string{"c1"}}},
		escolas:     []bubble.Escola{escolaOK("e1", "1", bubble.StatusConectada)},
		contratos:   []bubble.ContratoInstalacao{kitRI("c1")},
		importacoes: []bubble.ImportacaoEscola{{ID: "i1", INEP: "1", DataRelatorio: "2026-08-21T17:55:00.000Z"}},
	}
	got, err := bubble.MontarMes(f, agosto)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Itens) != 1 {
		t.Fatalf("itens=%d skips=%+v", len(got.Itens), got.Skips)
	}
	if d := got.Itens[0].Data.Format("2006-01-02"); d != "2026-08-20" {
		t.Fatalf("data = %s", d)
	}
}

func TestMontarMesEscolaConectadaSemDataDeConexaoViraSkip(t *testing.T) {
	// A escola já conectou, mas não há importação que diga quando. A previsão de
	// entrega da OSP não serve de reserva: ela arrastaria uma conexão de qualquer
	// mês passado para o mês puxado.
	f := &fonteFalsa{
		osps:      []bubble.OSP{{ID: "osp1", Status: "Nota Fiscal", PrevisaoEntrega: "2026-08-08T17:44:00.000Z", FRs: []string{"fr1"}}},
		folhas:    []bubble.FolhaOSP{{ID: "fr1", INEP: "1", EscolaID: "e1", OSPID: "osp1", ListaContratosInstalacao: []string{"c1"}}},
		escolas:   []bubble.Escola{escolaOK("e1", "1", bubble.StatusConectada)},
		contratos: []bubble.ContratoInstalacao{kitRI("c1")},
	}
	got, err := bubble.MontarMes(f, agosto)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Itens) != 0 {
		t.Fatalf("itens = %+v", got.Itens)
	}
	if len(got.Skips) != 1 || got.Skips[0].Motivo != bubble.SkipSemConexao {
		t.Fatalf("skips = %+v", got.Skips)
	}
}

func TestMontarMesEscolaNaoConectadaUsaAPrevisao(t *testing.T) {
	f := &fonteFalsa{
		osps:      []bubble.OSP{{ID: "osp1", Status: "Nota Fiscal", PrevisaoEntrega: "2026-08-08T17:44:00.000Z", FRs: []string{"fr1"}}},
		folhas:    []bubble.FolhaOSP{{ID: "fr1", INEP: "1", EscolaID: "e1", OSPID: "osp1", ListaContratosInstalacao: []string{"c1"}}},
		escolas:   []bubble.Escola{escolaOK("e1", "1", "Em planejamento")},
		contratos: []bubble.ContratoInstalacao{kitRI("c1")},
	}
	got, err := bubble.MontarMes(f, agosto)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Itens) != 1 {
		t.Fatalf("itens=%d skips=%+v", len(got.Itens), got.Skips)
	}
	if d := got.Itens[0].Data.Format("2006-01-02"); d != "2026-08-08" {
		t.Fatalf("data = %s", d)
	}
}

func TestMontarMesNaoUsaAListaFRDaOSP(t *testing.T) {
	// No Bubble a lista `FR` de dentro da OSP fica incompleta (OSP com
	// "FR esperadas" 8 e só 3 na lista). Quem manda é a folha, que aponta para a
	// OSP: a folha fora da lista tem que entrar do mesmo jeito.
	f := &fonteFalsa{
		osps: []bubble.OSP{{
			ID: "osp1", Status: "Nota Fiscal", PrevisaoEntrega: "2026-08-08T17:44:00.000Z",
			FRs: []string{"frNaLista"}, FREsperadas: 2,
		}},
		folhas: []bubble.FolhaOSP{
			{ID: "frNaLista", INEP: "1", EscolaID: "e1", OSPID: "osp1", ListaContratosInstalacao: []string{"c1"}},
			{ID: "frForaDaLista", INEP: "2", EscolaID: "e1", OSPID: "osp1", ListaContratosInstalacao: []string{"c1"}},
		},
		escolas:   []bubble.Escola{escolaOK("e1", "1", "Em planejamento")},
		contratos: []bubble.ContratoInstalacao{kitRI("c1")},
	}
	got, err := bubble.MontarMes(f, agosto)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Itens) != 2 {
		t.Fatalf("itens = %+v, skips = %+v", got.Itens, got.Skips)
	}
	ineps := map[string]bool{}
	for _, item := range got.Itens {
		ineps[item.INEP] = true
	}
	if !ineps["1"] || !ineps["2"] {
		t.Fatalf("INEPs = %v, quer os dois", ineps)
	}
}

func TestMontarMesSkipsDosFiltrosDeSempre(t *testing.T) {
	osp := func(id, fr string) bubble.OSP {
		return bubble.OSP{ID: id, Status: "Nota Fiscal", PrevisaoEntrega: "2026-08-08T17:44:00.000Z", FRs: []string{fr}}
	}
	f := &fonteFalsa{
		osps: []bubble.OSP{
			{ID: "ospRep", Status: bubble.StatusReprovado, PrevisaoEntrega: "2026-08-08T17:44:00.000Z", FRs: []string{"frRep"}},
			osp("ospSemINEP", "frSemINEP"),
			osp("ospSemKit", "frSemKit"),
			osp("ospSemFase", "frSemFase"),
			osp("ospSemReg", "frSemReg"),
		},
		folhas: []bubble.FolhaOSP{
			{ID: "frRep", INEP: "rep", EscolaID: "e1", OSPID: "ospRep", ListaContratosInstalacao: []string{"c1"}},
			{ID: "frSemINEP", INEP: "", EscolaID: "e1", OSPID: "ospSemINEP", ListaContratosInstalacao: []string{"c1"}},
			{ID: "frSemKit", INEP: "semkit", EscolaID: "e1", OSPID: "ospSemKit", ListaContratosInstalacao: []string{"cOutro"}},
			{ID: "frSemFase", INEP: "semfase", EscolaID: "eSemFase", OSPID: "ospSemFase", ListaContratosInstalacao: []string{"c1"}},
			{ID: "frSemReg", INEP: "semreg", EscolaID: "eSemReg", OSPID: "ospSemReg", ListaContratosInstalacao: []string{"c1"}},
		},
		escolas: []bubble.Escola{
			escolaOK("e1", "1", "Em planejamento"),
			{ID: "eSemFase", INEP: "semfase", UF: "PA", Regional: "Norte"},
			{ID: "eSemReg", INEP: "semreg", UF: "PA", Fase: "3"},
		},
		contratos: []bubble.ContratoInstalacao{
			kitRI("c1"),
			{ID: "cOutro", Descricao: "Kit Rede Externa", TipoDeObra: "1-OUTRA"},
		},
	}
	got, err := bubble.MontarMes(f, agosto)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Itens) != 0 {
		t.Fatalf("itens = %+v", got.Itens)
	}
	// OSP reprovada nem é buscada pela previsão; o filtro do mês a descarta na origem.
	quer := map[string]bubble.MotivoSkip{
		"":        bubble.SkipSemINEP,
		"semkit":  bubble.SkipSemKitRI,
		"semfase": bubble.SkipSemFase,
		"semreg":  bubble.SkipSemRegional,
	}
	for inep, motivo := range quer {
		if got := skipDe(t, got, inep); got != motivo {
			t.Fatalf("inep %q: motivo = %q, quer %q", inep, got, motivo)
		}
	}
}

func TestMontarMesDescartaItemComDataForaDoMes(t *testing.T) {
	// Previsão em agosto, mas a escola conectou em julho: a data do Registro
	// seria de julho, então o item não pertence a agosto.
	f := &fonteFalsa{
		osps:        []bubble.OSP{{ID: "osp1", Status: "Nota Fiscal", PrevisaoEntrega: "2026-08-08T17:44:00.000Z", FRs: []string{"fr1"}}},
		folhas:      []bubble.FolhaOSP{{ID: "fr1", INEP: "1", EscolaID: "e1", OSPID: "osp1", ListaContratosInstalacao: []string{"c1"}}},
		escolas:     []bubble.Escola{escolaOK("e1", "1", bubble.StatusConectada)},
		contratos:   []bubble.ContratoInstalacao{kitRI("c1")},
		importacoes: []bubble.ImportacaoEscola{{ID: "i1", INEP: "1", DataRelatorio: "2026-07-16T17:55:00.000Z"}},
	}
	got, err := bubble.MontarMes(f, agosto)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Itens) != 0 {
		t.Fatalf("itens = %+v", got.Itens)
	}
	if len(got.Skips) != 1 {
		t.Fatalf("skips = %+v", got.Skips)
	}
	skip := got.Skips[0]
	if skip.Motivo != bubble.SkipForaDoMes {
		t.Fatalf("motivo = %q", skip.Motivo)
	}
	if skip.OSPID != "osp1" || skip.FolhaID != "fr1" || skip.INEP != "1" {
		t.Fatalf("%+v", skip)
	}
	if strings.Contains(string(bubble.SkipForaDoMes), "previsão") {
		t.Fatalf("motivo fala de previsão de entrega: %q", bubble.SkipForaDoMes)
	}
}

func TestMontarMesMantemItemComDataDentroDoMes(t *testing.T) {
	f := &fonteFalsa{
		osps:        []bubble.OSP{{ID: "osp1", Status: "Nota Fiscal", PrevisaoEntrega: "2026-08-08T17:44:00.000Z", FRs: []string{"fr1"}}},
		folhas:      []bubble.FolhaOSP{{ID: "fr1", INEP: "1", EscolaID: "e1", OSPID: "osp1", ListaContratosInstalacao: []string{"c1"}}},
		escolas:     []bubble.Escola{escolaOK("e1", "1", bubble.StatusConectada)},
		contratos:   []bubble.ContratoInstalacao{kitRI("c1")},
		importacoes: []bubble.ImportacaoEscola{{ID: "i1", INEP: "1", DataRelatorio: "2026-09-01T17:55:00.000Z"}},
	}
	got, err := bubble.MontarMes(f, agosto)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Itens) != 1 || got.Itens[0].Data.Format("2006-01-02") != "2026-08-31" {
		t.Fatalf("itens = %+v skips = %+v", got.Itens, got.Skips)
	}
}

func TestMontarMesFolhaDeMesAnteriorEntraPelaConexao(t *testing.T) {
	// FR de julho (previsão em julho) cuja escola conectou em agosto.
	f := &fonteFalsa{
		osps:        []bubble.OSP{{ID: "osp1", Status: "Nota Fiscal", PrevisaoEntrega: "2026-07-10T17:44:00.000Z", FRs: []string{"fr1"}}},
		folhas:      []bubble.FolhaOSP{{ID: "fr1", INEP: "1", EscolaID: "e1", OSPID: "osp1", ListaContratosInstalacao: []string{"c1"}}},
		escolas:     []bubble.Escola{escolaOK("e1", "1", bubble.StatusConectada)},
		contratos:   []bubble.ContratoInstalacao{kitRI("c1")},
		importacoes: []bubble.ImportacaoEscola{{ID: "i1", INEP: "1", DataRelatorio: "2026-08-06T17:55:00.000Z"}},
	}
	got, err := bubble.MontarMes(f, agosto)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Itens) != 1 {
		t.Fatalf("itens=%d skips=%+v", len(got.Itens), got.Skips)
	}
	if d := got.Itens[0].Data.Format("2006-01-02"); d != "2026-08-05" {
		t.Fatalf("data = %s", d)
	}
	// E o mesmo dado puxado em julho não traz essa folha, porque a data é de agosto.
	emJulho, err := bubble.MontarMes(f, julho)
	if err != nil {
		t.Fatal(err)
	}
	if len(emJulho.Itens) != 0 {
		t.Fatalf("julho = %+v", emJulho.Itens)
	}
}

func TestMontarMesOSPNosDoisCaminhosGeraUmItem(t *testing.T) {
	f := &fonteFalsa{
		osps:        []bubble.OSP{{ID: "osp1", Status: "Nota Fiscal", PrevisaoEntrega: "2026-08-08T17:44:00.000Z", FRs: []string{"fr1"}}},
		folhas:      []bubble.FolhaOSP{{ID: "fr1", INEP: "1", EscolaID: "e1", OSPID: "osp1", ListaContratosInstalacao: []string{"c1"}}},
		escolas:     []bubble.Escola{escolaOK("e1", "1", bubble.StatusConectada)},
		contratos:   []bubble.ContratoInstalacao{kitRI("c1")},
		importacoes: []bubble.ImportacaoEscola{{ID: "i1", INEP: "1", DataRelatorio: "2026-08-21T17:55:00.000Z"}},
	}
	got, err := bubble.MontarMes(f, agosto)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Itens) != 1 {
		t.Fatalf("itens=%d %+v", len(got.Itens), got.Itens)
	}
}

func TestMontarMesDuasConexoesDoMesmoINEPNaMesmaDataGeramUmaLinha(t *testing.T) {
	f := &fonteFalsa{
		osps:      []bubble.OSP{{ID: "osp1", Status: "Nota Fiscal", PrevisaoEntrega: "2026-07-10T17:44:00.000Z", FRs: []string{"fr1"}}},
		folhas:    []bubble.FolhaOSP{{ID: "fr1", INEP: "1", EscolaID: "e1", OSPID: "osp1", ListaContratosInstalacao: []string{"c1"}}},
		escolas:   []bubble.Escola{escolaOK("e1", "1", bubble.StatusConectada)},
		contratos: []bubble.ContratoInstalacao{kitRI("c1")},
		importacoes: []bubble.ImportacaoEscola{
			{ID: "i1", INEP: "1", DataRelatorio: "2026-08-06T10:00:00.000Z"},
			{ID: "i2", INEP: "1", DataRelatorio: "2026-08-06T20:00:00.000Z"},
		},
	}
	got, err := bubble.MontarMes(f, agosto)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Itens) != 1 || got.Itens[0].Data.Format("2006-01-02") != "2026-08-05" {
		t.Fatalf("itens = %+v", got.Itens)
	}
}

func TestMontarMesUsaImportacaoMaisRecente(t *testing.T) {
	f := &fonteFalsa{
		osps:      []bubble.OSP{{ID: "osp1", Status: "Nota Fiscal", PrevisaoEntrega: "2026-07-10T17:44:00.000Z", FRs: []string{"fr1"}}},
		folhas:    []bubble.FolhaOSP{{ID: "fr1", INEP: "1", EscolaID: "e1", OSPID: "osp1", ListaContratosInstalacao: []string{"c1"}}},
		escolas:   []bubble.Escola{escolaOK("e1", "1", bubble.StatusConectada)},
		contratos: []bubble.ContratoInstalacao{kitRI("c1")},
		importacoes: []bubble.ImportacaoEscola{
			{ID: "i1", INEP: "1", DataRelatorio: "2026-08-06T10:00:00.000Z"},
			{ID: "i2", INEP: "1", DataRelatorio: "2026-08-20T10:00:00.000Z"},
		},
	}
	got, err := bubble.MontarMes(f, agosto)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Itens) != 1 || got.Itens[0].Data.Format("2006-01-02") != "2026-08-19" {
		t.Fatalf("itens = %+v", got.Itens)
	}
}

func TestMontarMesConexaoPassaPelosMesmosFiltros(t *testing.T) {
	// Todas as folhas abaixo só são alcançadas pelo caminho de conexão:
	// previsão de entrega em julho, conexão em agosto.
	f := &fonteFalsa{
		osps: []bubble.OSP{
			{ID: "ospRep", Status: bubble.StatusReprovado, PrevisaoEntrega: "2026-07-10T17:44:00.000Z", FRs: []string{"frRep"}},
			{ID: "ospSemKit", Status: "Nota Fiscal", PrevisaoEntrega: "2026-07-10T17:44:00.000Z", FRs: []string{"frSemKit"}},
			{ID: "ospSemFase", Status: "Nota Fiscal", PrevisaoEntrega: "2026-07-10T17:44:00.000Z", FRs: []string{"frSemFase"}},
			{ID: "ospSemReg", Status: "Nota Fiscal", PrevisaoEntrega: "2026-07-10T17:44:00.000Z", FRs: []string{"frSemReg", "frSemINEP"}},
		},
		folhas: []bubble.FolhaOSP{
			{ID: "frRep", INEP: "rep", EscolaID: "eRep", OSPID: "ospRep", ListaContratosInstalacao: []string{"c1"}},
			{ID: "frSemKit", INEP: "semkit", EscolaID: "eSemKit", OSPID: "ospSemKit", ListaContratosInstalacao: []string{"cOutro"}},
			{ID: "frSemFase", INEP: "semfase", EscolaID: "eSemFase", OSPID: "ospSemFase", ListaContratosInstalacao: []string{"c1"}},
			{ID: "frSemReg", INEP: "semreg", EscolaID: "eSemReg", OSPID: "ospSemReg", ListaContratosInstalacao: []string{"c1"}},
			// folha irmã da mesma OSP, alcançada só por ela: INEP vazio
			{ID: "frSemINEP", INEP: "", EscolaID: "eSemReg", OSPID: "ospSemReg", ListaContratosInstalacao: []string{"c1"}},
		},
		escolas: []bubble.Escola{
			escolaOK("eRep", "rep", bubble.StatusConectada),
			escolaOK("eSemKit", "semkit", bubble.StatusConectada),
			{ID: "eSemFase", INEP: "semfase", UF: "PA", Regional: "Norte", StatusGeral: bubble.StatusConectada},
			{ID: "eSemReg", INEP: "semreg", UF: "PA", Fase: "3", StatusGeral: bubble.StatusConectada},
		},
		contratos: []bubble.ContratoInstalacao{
			kitRI("c1"),
			{ID: "cOutro", Descricao: "Kit Rede Externa", TipoDeObra: "1-OUTRA"},
		},
		importacoes: []bubble.ImportacaoEscola{
			{ID: "i1", INEP: "rep", DataRelatorio: "2026-08-06T10:00:00.000Z"},
			{ID: "i2", INEP: "semkit", DataRelatorio: "2026-08-06T10:00:00.000Z"},
			{ID: "i3", INEP: "semfase", DataRelatorio: "2026-08-06T10:00:00.000Z"},
			{ID: "i4", INEP: "semreg", DataRelatorio: "2026-08-06T10:00:00.000Z"},
		},
	}
	got, err := bubble.MontarMes(f, agosto)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Itens) != 0 {
		t.Fatalf("itens = %+v", got.Itens)
	}
	quer := map[string]bubble.MotivoSkip{
		"rep":     bubble.SkipOSPReprovada,
		"semkit":  bubble.SkipSemKitRI,
		"semfase": bubble.SkipSemFase,
		"semreg":  bubble.SkipSemRegional,
		"":        bubble.SkipSemINEP,
	}
	for inep, motivo := range quer {
		if m := skipDe(t, got, inep); m != motivo {
			t.Fatalf("inep %q: motivo = %q, quer %q", inep, m, motivo)
		}
	}
	// A OSP Reprovada não conta como OSP trazida pela conexão.
	if got.Resumo.OSPsPorConexao != 3 {
		t.Fatalf("osps por conexão = %d, quer 3 (a Reprovada é descartada)", got.Resumo.OSPsPorConexao)
	}
}

func TestMontarMesEscolaNaoConectadaSoEntraPelaPrevisao(t *testing.T) {
	// Escola não Conectada com importação em agosto: a conexão não conta,
	// e a previsão de julho mantém a folha fora de agosto.
	f := &fonteFalsa{
		osps:        []bubble.OSP{{ID: "osp1", Status: "Nota Fiscal", PrevisaoEntrega: "2026-07-10T17:44:00.000Z", FRs: []string{"fr1"}}},
		folhas:      []bubble.FolhaOSP{{ID: "fr1", INEP: "1", EscolaID: "e1", OSPID: "osp1", ListaContratosInstalacao: []string{"c1"}}},
		escolas:     []bubble.Escola{escolaOK("e1", "1", "Em planejamento")},
		contratos:   []bubble.ContratoInstalacao{kitRI("c1")},
		importacoes: []bubble.ImportacaoEscola{{ID: "i1", INEP: "1", DataRelatorio: "2026-08-06T10:00:00.000Z"}},
	}
	emAgosto, err := bubble.MontarMes(f, agosto)
	if err != nil {
		t.Fatal(err)
	}
	if len(emAgosto.Itens) != 0 {
		t.Fatalf("agosto = %+v", emAgosto.Itens)
	}
	emJulho, err := bubble.MontarMes(f, julho)
	if err != nil {
		t.Fatal(err)
	}
	if len(emJulho.Itens) != 1 || emJulho.Itens[0].Data.Format("2006-01-02") != "2026-07-10" {
		t.Fatalf("julho = %+v", emJulho.Itens)
	}
}

func TestMontarMesErroDeImportacoesDoMesPropaga(t *testing.T) {
	boom := errors.New("data api caiu")
	f := &fonteFalsa{
		osps:                 []bubble.OSP{{ID: "osp1", Status: "NF", PrevisaoEntrega: "2026-08-08T17:44:00.000Z", FRs: []string{"fr1"}}},
		folhas:               []bubble.FolhaOSP{{ID: "fr1", INEP: "1", EscolaID: "e1", ListaContratosInstalacao: []string{"c1"}}},
		escolas:              []bubble.Escola{escolaOK("e1", "1", "Em planejamento")},
		contratos:            []bubble.ContratoInstalacao{kitRI("c1")},
		erroImportacoesDoMes: boom,
	}
	got, err := bubble.MontarMes(f, agosto)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if len(got.Itens) != 0 {
		t.Fatalf("resultado parcial: %+v", got.Itens)
	}
}

func TestMontarMesResumoContaAsDuasOrigens(t *testing.T) {
	f := &fonteFalsa{
		osps: []bubble.OSP{
			// entra pela previsão de agosto
			{ID: "ospPrev", Status: "Nota Fiscal", PrevisaoEntrega: "2026-08-08T17:44:00.000Z", FRs: []string{"frPrev"}},
			// entra pela conexão de agosto, com previsão em julho
			{ID: "ospConex", Status: "Nota Fiscal", PrevisaoEntrega: "2026-07-10T17:44:00.000Z", FRs: []string{"frConex"}},
			// previsão em agosto, mas conectou em julho: cai no filtro de data
			{ID: "ospFora", Status: "Nota Fiscal", PrevisaoEntrega: "2026-08-20T17:44:00.000Z", FRs: []string{"frFora"}},
		},
		folhas: []bubble.FolhaOSP{
			{ID: "frPrev", INEP: "prev", EscolaID: "ePrev", OSPID: "ospPrev", ListaContratosInstalacao: []string{"c1"}},
			{ID: "frConex", INEP: "conex", EscolaID: "eConex", OSPID: "ospConex", ListaContratosInstalacao: []string{"c1"}},
			{ID: "frFora", INEP: "fora", EscolaID: "eFora", OSPID: "ospFora", ListaContratosInstalacao: []string{"c1"}},
		},
		escolas: []bubble.Escola{
			escolaOK("ePrev", "prev", "Em planejamento"),
			escolaOK("eConex", "conex", bubble.StatusConectada),
			escolaOK("eFora", "fora", bubble.StatusConectada),
		},
		contratos: []bubble.ContratoInstalacao{kitRI("c1")},
		importacoes: []bubble.ImportacaoEscola{
			{ID: "i1", INEP: "conex", DataRelatorio: "2026-08-06T10:00:00.000Z"},
			{ID: "i2", INEP: "fora", DataRelatorio: "2026-07-16T10:00:00.000Z"},
		},
	}
	got, err := bubble.MontarMes(f, agosto)
	if err != nil {
		t.Fatal(err)
	}
	quer := bubble.PuxarResumo{OSPsPorPrevisao: 2, OSPsPorConexao: 1, OSPsUnicas: 3, ItensForaDoMes: 1}
	if got.Resumo != quer {
		t.Fatalf("resumo = %+v, quer %+v", got.Resumo, quer)
	}
}

func TestMontarMesResumoComConexaoVazia(t *testing.T) {
	f := &fonteFalsa{
		osps:      []bubble.OSP{{ID: "osp1", Status: "Nota Fiscal", PrevisaoEntrega: "2026-08-08T17:44:00.000Z", FRs: []string{"fr1"}}},
		folhas:    []bubble.FolhaOSP{{ID: "fr1", INEP: "1", EscolaID: "e1", OSPID: "osp1", ListaContratosInstalacao: []string{"c1"}}},
		escolas:   []bubble.Escola{escolaOK("e1", "1", "Em planejamento")},
		contratos: []bubble.ContratoInstalacao{kitRI("c1")},
	}
	got, err := bubble.MontarMes(f, agosto)
	if err != nil {
		t.Fatal(err)
	}
	quer := bubble.PuxarResumo{OSPsPorPrevisao: 1, OSPsPorConexao: 0, OSPsUnicas: 1, ItensForaDoMes: 0}
	if got.Resumo != quer {
		t.Fatalf("resumo = %+v, quer %+v", got.Resumo, quer)
	}
}
