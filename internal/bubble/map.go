package bubble

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/wellington/pcp_processamento/internal/domain"
)

const (
	TipoObraRedeInterna = "4-IMPLANTAÇÃO_DE_REDE_INTERNA"
	StatusReprovado     = "Reprovado"
	StatusConectada     = "Conectada"
)

var regionalNomes = map[string]string{
	"NO":    "Norte",
	"NE-I":  "Nordeste I",
	"NE-II": "Nordeste II",
	"SUSE":  "Sudeste/Centro-Sul",
	"COSE":  "Centro-Oeste/Minas",
}

// MotivoSkip is why a folha does not become Programado.
type MotivoSkip string

const (
	SkipSemINEP      MotivoSkip = "sem INEP"
	SkipSemEscola    MotivoSkip = "sem escola"
	SkipSemFase      MotivoSkip = "escola sem fase"
	SkipSemRegional  MotivoSkip = "escola sem regional"
	SkipOSPReprovada MotivoSkip = "OSP reprovada"
	SkipForaDoMes    MotivoSkip = "data do Programado fora do mês"
	SkipSemPrevisao  MotivoSkip = "OSP sem previsão de entrega"
	SkipSemConexao   MotivoSkip = "escola conectada sem data de conexão"
	SkipSemKitRI     MotivoSkip = "folha sem contrato kit de implantação de rede interna"
	SkipSemFolha     MotivoSkip = "folha não encontrada"
)

func resolveRegional(s string) (sigla, nome string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	for sig, n := range regionalNomes {
		if strings.EqualFold(s, sig) || strings.EqualFold(s, n) {
			return sig, n
		}
	}
	return s, ""
}

func locBR() *time.Location {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return time.UTC
	}
	return loc
}

// DataPrevisaoEntrega is the civil date of OSP.Previsão de entrega (Brazil).
func DataPrevisaoEntrega(osp OSP) (time.Time, bool) {
	return civilDate(osp.PrevisaoEntrega)
}

func civilDate(s string) (time.Time, bool) {
	return civilDateComAtraso(s, 0)
}

func civilDateComAtraso(s string, atraso time.Duration) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05.000Z", s)
	}
	if err != nil {
		return time.Time{}, false
	}
	local := t.Add(atraso).In(locBR())
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC), true
}

func EscolaConectada(escola *Escola) bool {
	if escola == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(escola.StatusGeral), StatusConectada)
}

func OSPProvisoria(osp OSP) bool {
	return osp.OSnum == nil || *osp.OSnum == 0
}

// atrasoDataRelatorio é o quanto data_relatorio adianta a conexão: o relatório é
// carimbado em D+1, exatamente 24h depois do lançamento da importação. Conferido
// contra o Report - PCP - Escolas Conectadas (104 de 104 registros com 24h justas).
const atrasoDataRelatorio = 24 * time.Hour

// DataConexao is the civil date the connection was recorded: data_relatorio minus
// the D+1 stamp of the report.
func DataConexao(imp *ImportacaoEscola) (time.Time, bool) {
	if imp == nil {
		return time.Time{}, false
	}
	return civilDateComAtraso(imp.DataRelatorio, -atrasoDataRelatorio)
}

// ImportacaoComRelatorio returns the row with the latest data_relatorio, if any.
func ImportacaoComRelatorio(rows []ImportacaoEscola) *ImportacaoEscola {
	var best *ImportacaoEscola
	var bestDate time.Time
	for i := range rows {
		d, ok := DataConexao(&rows[i])
		if !ok {
			continue
		}
		if best == nil || d.After(bestDate) {
			row := rows[i]
			best = &row
			bestDate = d
		}
	}
	return best
}

// DataProgramado é a data que vai para o Registro: a data da conexão quando a
// escola está Conectada, senão a previsão de entrega da OSP.
//
// Escola Conectada só entra pela conexão. Se a data da conexão não dá para
// determinar, o item vira skip em vez de cair na previsão: a previsão de entrega
// de uma escola que já conectou não diz nada sobre o mês dela, e usá-la arrasta
// conexões antigas para o mês que está sendo puxado.
func DataProgramado(osp OSP, escola *Escola, imp *ImportacaoEscola) (time.Time, MotivoSkip) {
	if EscolaConectada(escola) {
		if d, ok := DataConexao(imp); ok {
			return d, ""
		}
		return time.Time{}, SkipSemConexao
	}
	if d, ok := DataPrevisaoEntrega(osp); ok {
		return d, ""
	}
	return time.Time{}, SkipSemPrevisao
}

func OSPNaoReprovada(osp OSP) bool {
	return !strings.EqualFold(strings.TrimSpace(osp.Status), StatusReprovado)
}

// DataNoMes says whether a civil date belongs to the civil month.
func DataNoMes(data, mes time.Time) bool {
	return data.Year() == mes.Year() && data.Month() == mes.Month()
}

func OSPNoMes(osp OSP, mes time.Time) bool {
	d, ok := DataPrevisaoEntrega(osp)
	if !ok {
		return false
	}
	return DataNoMes(d, mes)
}

// MesCivil parses YYYY-MM, or the current month in America/Sao_Paulo when s is empty.
func MesCivil(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		loc, err := time.LoadLocation("America/Sao_Paulo")
		if err != nil {
			loc = time.UTC
		}
		now := time.Now().In(loc)
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC), nil
	}
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("mês inválido %q (use YYYY-MM)", s)
	}
	return t, nil
}

// ConstraintsOSPMes is the Data API constraint JSON: previsão no mês e status ≠ Reprovado.
func ConstraintsOSPMes(mes time.Time) string {
	cons := append(constraintsCampoNoMes("Previsão de entrega", mes, 0),
		map[string]string{"key": "status", "constraint_type": "not equal", "value": StatusReprovado})
	b, _ := json.Marshal(cons)
	return string(b)
}

// constraintsCampoNoMes recorta um campo de data no mês civil em America/Sao_Paulo.
// atraso desloca a janela quando o campo é carimbado depois do evento.
func constraintsCampoNoMes(campo string, mes time.Time, atraso time.Duration) []map[string]string {
	ini := time.Date(mes.Year(), mes.Month(), 1, 0, 0, 0, 0, locBR()).Add(atraso)
	fim := ini.AddDate(0, 1, 0)
	return []map[string]string{
		{"key": campo, "constraint_type": "greater than", "value": ini.UTC().Add(-time.Second).Format("2006-01-02T15:04:05.000Z")},
		{"key": campo, "constraint_type": "less than", "value": fim.UTC().Format("2006-01-02T15:04:05.000Z")},
	}
}

// ConstraintsImportacaoMes is the Data API constraint JSON: conexão dentro do mês
// civil. A janela é deslocada em +1 dia porque data_relatorio é carimbado em D+1.
func ConstraintsImportacaoMes(mes time.Time) string {
	b, _ := json.Marshal(constraintsCampoNoMes("data_relatorio", mes, atrasoDataRelatorio))
	return string(b)
}

// ConstraintsFolhasINEPs filtra fr_osp pela chave INEP da folha (não a da escola).
// Lista vazia continua sendo uma constraint "in" de lista vazia, que não casa nada,
// em vez de virar consulta sem constraint (que devolveria a coleção inteira).
func ConstraintsFolhasINEPs(ineps []string) string {
	if ineps == nil {
		ineps = []string{}
	}
	b, _ := json.Marshal([]map[string]any{
		{"key": "INEP", "constraint_type": "in", "value": ineps},
	})
	return string(b)
}

// ConstraintsFolhasOSPs filtra fr_osp pela OSP a que a folha pertence. É o caminho
// de ida: a folha é que aponta para a OSP. A lista `FR` dentro da OSP não serve de
// índice — ela fica incompleta (OSP com "FR esperadas" 8 e só 3 na lista), e uma
// folha fora dela ficaria invisível. Lista vazia continua sendo um "in" de lista
// vazia, que não casa nada, em vez de virar consulta sem constraint.
func ConstraintsFolhasOSPs(ospIDs []string) string {
	if ospIDs == nil {
		ospIDs = []string{}
	}
	b, _ := json.Marshal([]map[string]any{
		{"key": "OSP", "constraint_type": "in", "value": ospIDs},
	})
	return string(b)
}

func ContratoKitRedeInterna(c ContratoInstalacao) bool {
	if c.TipoDeObra != TipoObraRedeInterna {
		return false
	}
	return strings.Contains(strings.ToLower(c.Descricao), "kit")
}

func FolhaTemKitRI(folha FolhaOSP, contratos map[string]ContratoInstalacao) bool {
	for _, id := range folha.ListaContratosInstalacao {
		if ContratoKitRedeInterna(contratos[id]) {
			return true
		}
	}
	return false
}

// ProgramadoDaFolha monta o ItemCarga quando a OSP do mês tem previsão de entrega
// e a folha tem contrato kit de implantação de rede interna.
func ProgramadoDaFolha(osp OSP, folha FolhaOSP, escola *Escola, contratos map[string]ContratoInstalacao, imp *ImportacaoEscola) (domain.ItemCarga, MotivoSkip) {
	if !OSPNaoReprovada(osp) {
		return domain.ItemCarga{}, SkipOSPReprovada
	}
	if strings.TrimSpace(folha.INEP) == "" {
		return domain.ItemCarga{}, SkipSemINEP
	}
	if !FolhaTemKitRI(folha, contratos) {
		return domain.ItemCarga{}, SkipSemKitRI
	}
	if escola == nil {
		return domain.ItemCarga{}, SkipSemEscola
	}
	fase := strings.TrimSpace(escola.Fase)
	if fase == "" {
		return domain.ItemCarga{}, SkipSemFase
	}
	sigla, nome := resolveRegional(escola.Regional)
	if sigla == "" {
		return domain.ItemCarga{}, SkipSemRegional
	}
	data, motivo := DataProgramado(osp, escola, imp)
	if motivo != "" {
		return domain.ItemCarga{}, motivo
	}
	uf := strings.TrimSpace(escola.UF)
	if uf == "" {
		uf = strings.TrimSpace(folha.UF)
	}
	prov := OSPProvisoria(osp)
	return domain.ItemCarga{
		Tipo:           domain.TipoProgramado,
		Data:           data,
		Fase:           fase,
		Regional:       sigla,
		RegionalNome:   nome,
		UF:             uf,
		INEP:           strings.TrimSpace(folha.INEP),
		FornecedorNome: strings.TrimSpace(escola.FornecedorRI),
		FornecedorCNPJ: strings.TrimSpace(escola.CNPJFornecedorRI),
		Quantidade:     1,
		Provisoria:     &prov,
	}, ""
}

// CamposDaJuncao is the inspectable join of folha + escola.
type CamposDaJuncao struct {
	FolhaID        string
	OSPID          string
	EscolaID       string
	INEP           string
	UF             string
	Fase           string
	Regional       string
	RegionalNome   string
	FornecedorNome string
	FornecedorCNPJ string
	FolhaStatus    string
	FolhaTipo      string
	Quantidade     int
}

func JuncaoFolhaEscola(folha FolhaOSP, escola *Escola) CamposDaJuncao {
	out := CamposDaJuncao{
		FolhaID:     folha.ID,
		OSPID:       folha.OSPID,
		EscolaID:    folha.EscolaID,
		INEP:        strings.TrimSpace(folha.INEP),
		UF:          strings.TrimSpace(folha.UF),
		FolhaStatus: folha.Status,
		FolhaTipo:   folha.Tipo,
		Quantidade:  1,
	}
	if escola == nil {
		return out
	}
	if v := strings.TrimSpace(escola.INEP); v != "" {
		out.INEP = v
	}
	if v := strings.TrimSpace(escola.UF); v != "" {
		out.UF = v
	}
	out.EscolaID = escola.ID
	out.Fase = strings.TrimSpace(escola.Fase)
	out.Regional, out.RegionalNome = resolveRegional(escola.Regional)
	out.FornecedorNome = strings.TrimSpace(escola.FornecedorRI)
	out.FornecedorCNPJ = strings.TrimSpace(escola.CNPJFornecedorRI)
	return out
}
