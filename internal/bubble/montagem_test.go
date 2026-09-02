package bubble_test

import (
	"strings"
	"testing"
	"time"

	"github.com/wellington/pcp_processamento/internal/bubble"
)

// fonteFalsa é a porta de busca em memória: os mesmos dados que o Bubble
// devolveria, sem HTTP. Os filtros que a Data API faria por constraint
// (mês da previsão, mês da data_relatorio, INEP, IDs) são feitos aqui.
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
		if noMes(imp.DataRelatorio, mes) {
			out = append(out, imp)
		}
	}
	return out, nil
}

func (f *fonteFalsa) FolhasPorIDs(ids []string) (map[string]bubble.FolhaOSP, error) {
	quer := conjunto(ids)
	out := map[string]bubble.FolhaOSP{}
	for _, folha := range f.folhas {
		if _, ok := quer[folha.ID]; ok {
			out[folha.ID] = folha
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

func noMes(data string, mes time.Time) bool {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(data))
	if err != nil {
		return false
	}
	loc, errLoc := time.LoadLocation("America/Sao_Paulo")
	if errLoc == nil {
		t = t.In(loc)
	}
	return t.Year() == mes.Year() && t.Month() == mes.Month()
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
		importacoes: []bubble.ImportacaoEscola{{ID: "i1", INEP: "1", DataRelatorio: "2026-08-20T17:55:00.000Z"}},
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

func TestMontarMesEscolaConectadaSemDataRelatorioCaiNaPrevisao(t *testing.T) {
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
	if len(got.Itens) != 1 {
		t.Fatalf("itens=%d skips=%+v", len(got.Itens), got.Skips)
	}
	if d := got.Itens[0].Data.Format("2006-01-02"); d != "2026-08-08" {
		t.Fatalf("data = %s", d)
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
			{ID: "frRep", INEP: "rep", EscolaID: "e1", ListaContratosInstalacao: []string{"c1"}},
			{ID: "frSemINEP", INEP: "", EscolaID: "e1", ListaContratosInstalacao: []string{"c1"}},
			{ID: "frSemKit", INEP: "semkit", EscolaID: "e1", ListaContratosInstalacao: []string{"cOutro"}},
			{ID: "frSemFase", INEP: "semfase", EscolaID: "eSemFase", ListaContratosInstalacao: []string{"c1"}},
			{ID: "frSemReg", INEP: "semreg", EscolaID: "eSemReg", ListaContratosInstalacao: []string{"c1"}},
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
