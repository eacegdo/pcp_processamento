package bubble

import (
	"log"
	"strings"
	"time"

	"github.com/wellington/pcp_processamento/internal/domain"
)

// FonteBusca é a porta de busca no Bubble que a montagem do mês precisa.
// O Client a satisfaz; testes de comportamento usam uma fonte em memória.
type FonteBusca interface {
	// OSPsDoMes traz as OSPs com Previsão de entrega no mês, status ≠ Reprovado.
	OSPsDoMes(mes time.Time) ([]OSP, error)
	// OSPsPorIDs traz as OSPs desses IDs.
	OSPsPorIDs(ids []string) (map[string]OSP, error)
	// ImportacoesDoMes traz as importações de escola com data_relatorio no mês.
	ImportacoesDoMes(mes time.Time) ([]ImportacaoEscola, error)
	// FolhasPorIDs traz as Folhas de Registro desses IDs.
	FolhasPorIDs(ids []string) (map[string]FolhaOSP, error)
	// FolhasPorINEPs traz as Folhas de Registro cujo INEP (o da folha) está na lista.
	FolhasPorINEPs(ineps []string) ([]FolhaOSP, error)
	// ContratosPorIDs traz os contratos de instalação desses IDs.
	ContratosPorIDs(ids []string) (map[string]ContratoInstalacao, error)
	// EscolasPorIDs traz as escolas desses IDs.
	EscolasPorIDs(ids []string) (map[string]Escola, error)
	// ImportacoesPorINEPs traz, por INEP, a importação de data_relatorio mais recente.
	ImportacoesPorINEPs(ineps []string) (map[string]*ImportacaoEscola, error)
}

// MontarMes monta o Programado do mês civil sobre a porta de busca, sem falar HTTP.
func MontarMes(fonte FonteBusca, mes time.Time) (Puxado, error) {
	ospsPrevisao, err := fonte.OSPsDoMes(mes)
	if err != nil {
		return Puxado{}, err
	}
	ospsConexao, err := ospsDasConexoesDoMes(fonte, mes)
	if err != nil {
		return Puxado{}, err
	}
	osps := unirOSPs(ospsPrevisao, ospsConexao)

	frIDs := make([]string, 0)
	for _, osp := range osps {
		frIDs = append(frIDs, osp.FRs...)
	}
	log.Printf("puxar: %d OSPs por previsão, %d por conexão, %d após dedupe; %d folhas; buscando em lote",
		len(ospsPrevisao), len(ospsConexao), len(osps), len(uniqueNonEmpty(frIDs)))

	folhas, err := fonte.FolhasPorIDs(frIDs)
	if err != nil {
		return Puxado{}, err
	}

	var contratoIDs, escolaIDs []string
	for _, folha := range folhas {
		contratoIDs = append(contratoIDs, folha.ListaContratosInstalacao...)
		if id := strings.TrimSpace(folha.EscolaID); id != "" {
			escolaIDs = append(escolaIDs, id)
		}
	}
	contratos, err := fonte.ContratosPorIDs(contratoIDs)
	if err != nil {
		return Puxado{}, err
	}
	escolas, err := fonte.EscolasPorIDs(escolaIDs)
	if err != nil {
		return Puxado{}, err
	}

	var ineps []string
	for _, esc := range escolas {
		if EscolaConectada(&esc) {
			if inep := strings.TrimSpace(esc.INEP); inep != "" {
				ineps = append(ineps, inep)
			}
		}
	}
	// Folha INEP can differ from escola INEP; still look up by folha INEP when connected.
	for _, folha := range folhas {
		esc, ok := escolas[strings.TrimSpace(folha.EscolaID)]
		if !ok {
			continue
		}
		if EscolaConectada(&esc) {
			if inep := strings.TrimSpace(folha.INEP); inep != "" {
				ineps = append(ineps, inep)
			}
		}
	}
	imps, err := fonte.ImportacoesPorINEPs(ineps)
	if err != nil {
		return Puxado{}, err
	}

	out := Puxado{Itens: make([]domain.ItemCarga, 0), Skips: make([]PuxarSkip, 0)}
	for _, osp := range osps {
		for _, fid := range osp.FRs {
			fid = strings.TrimSpace(fid)
			if fid == "" {
				continue
			}
			folha, ok := folhas[fid]
			if !ok {
				out.Skips = append(out.Skips, PuxarSkip{OSPID: osp.ID, FolhaID: fid, Motivo: "folha não encontrada"})
				continue
			}
			folhaContratos := map[string]ContratoInstalacao{}
			for _, cid := range folha.ListaContratosInstalacao {
				if ct, ok := contratos[strings.TrimSpace(cid)]; ok {
					folhaContratos[cid] = ct
				}
			}
			var escola *Escola
			if id := strings.TrimSpace(folha.EscolaID); id != "" {
				if esc, ok := escolas[id]; ok {
					escola = &esc
				}
			}
			inep := strings.TrimSpace(folha.INEP)
			var imp *ImportacaoEscola
			if EscolaConectada(escola) && inep != "" {
				imp = imps[inep]
			}
			item, skip := ProgramadoDaFolha(osp, folha, escola, folhaContratos, imp)
			if skip != "" {
				out.Skips = append(out.Skips, PuxarSkip{OSPID: osp.ID, FolhaID: folha.ID, INEP: inep, Motivo: skip})
				continue
			}
			// O mês é o da data que vai ser gravada no Registro PCP, não o da
			// previsão de entrega: item com data de outro mês não pertence a este.
			if !DataNoMes(item.Data, mes) {
				out.Skips = append(out.Skips, PuxarSkip{OSPID: osp.ID, FolhaID: folha.ID, INEP: inep, Motivo: SkipForaDoMes})
				continue
			}
			out.Itens = append(out.Itens, item)
		}
	}
	return out, nil
}

// ospsDasConexoesDoMes percorre o caminho novo: importações com data_relatorio no
// mês → INEPs distintos → Folhas de Registro desses INEPs → as OSPs dessas folhas.
func ospsDasConexoesDoMes(fonte FonteBusca, mes time.Time) ([]OSP, error) {
	imps, err := fonte.ImportacoesDoMes(mes)
	if err != nil {
		return nil, err
	}

	var ineps []string
	for _, imp := range imps {
		d, ok := civilDate(imp.DataRelatorio)
		if !ok || !DataNoMes(d, mes) {
			continue
		}
		ineps = append(ineps, imp.INEP)
	}
	ineps = uniqueNonEmpty(ineps)
	if len(ineps) == 0 {
		return nil, nil
	}

	folhas, err := fonte.FolhasPorINEPs(ineps)
	if err != nil {
		return nil, err
	}
	var ospIDs []string
	for _, folha := range folhas {
		ospIDs = append(ospIDs, folha.OSPID)
	}
	ospIDs = uniqueNonEmpty(ospIDs)
	if len(ospIDs) == 0 {
		return nil, nil
	}

	porID, err := fonte.OSPsPorIDs(ospIDs)
	if err != nil {
		return nil, err
	}
	out := make([]OSP, 0, len(porID))
	for _, id := range ospIDs {
		if osp, ok := porID[id]; ok {
			out = append(out, osp)
		}
	}
	return out, nil
}

// unirOSPs junta os dois caminhos sem duplicar por _id, mantendo a ordem
// de previsão primeiro e depois conexão.
func unirOSPs(previsao, conexao []OSP) []OSP {
	out := make([]OSP, 0, len(previsao)+len(conexao))
	vistos := make(map[string]struct{}, len(previsao)+len(conexao))
	for _, grupo := range [][]OSP{previsao, conexao} {
		for _, osp := range grupo {
			id := strings.TrimSpace(osp.ID)
			if id == "" {
				continue
			}
			if _, ok := vistos[id]; ok {
				continue
			}
			vistos[id] = struct{}{}
			out = append(out, osp)
		}
	}
	return out
}
