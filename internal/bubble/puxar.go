package bubble

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/wellington/pcp_processamento/internal/domain"
)

type PuxarSkip struct {
	OSPID   string
	FolhaID string
	INEP    string
	Motivo  MotivoSkip
}

type Puxado struct {
	Itens []domain.ItemCarga
	Skips []PuxarSkip
}

type jsonItem struct {
	Data           string `json:"data"`
	Fase           string `json:"fase"`
	Regional       string `json:"regional"`
	UF             string `json:"uf"`
	INEP           string `json:"inep"`
	FornecedorNome string `json:"fornecedor_nome"`
	FornecedorCNPJ string `json:"fornecedor_cnpj"`
	Quantidade     int    `json:"quantidade"`
	Provisoria     bool   `json:"provisoria"`
	Origem         string `json:"origem,omitempty"`
}

// EncodeProgramadoJSON is the payload of POST /v1/programado.
func EncodeProgramadoJSON(items []domain.ItemCarga) ([]byte, error) {
	rows := make([]jsonItem, 0, len(items))
	for _, item := range items {
		prov := false
		if item.Provisoria != nil {
			prov = *item.Provisoria
		}
		rows = append(rows, jsonItem{
			Data:           item.Data.Format("02/01/2006"),
			Fase:           item.Fase,
			Regional:       item.Regional,
			UF:             item.UF,
			INEP:           item.INEP,
			FornecedorNome: item.FornecedorNome,
			FornecedorCNPJ: item.FornecedorCNPJ,
			Quantidade:     item.Quantidade,
			Provisoria:     prov,
			Origem:         item.Origem,
		})
	}
	return json.MarshalIndent(rows, "", "  ")
}

// PuxarMes monta o Programado do mês civil a partir da Data API (OSP, FR, kit RI, escola, importação).
func (c *Client) PuxarMes(mes time.Time) (Puxado, error) {
	osps, err := c.listOSPsDoMes(mes)
	if err != nil {
		return Puxado{}, err
	}

	escolas := map[string]Escola{}
	contratos := map[string]ContratoInstalacao{}
	imps := map[string]*ImportacaoEscola{}
	impDone := map[string]bool{}

	out := Puxado{Itens: make([]domain.ItemCarga, 0), Skips: make([]PuxarSkip, 0)}
	for _, osp := range osps {
		for _, fid := range osp.FRs {
			folha, err := c.GetFolhaOSP(fid)
			if err != nil {
				return Puxado{}, fmt.Errorf("fr_osp %s: %w", fid, err)
			}
			folhaContratos := map[string]ContratoInstalacao{}
			for _, cid := range folha.ListaContratosInstalacao {
				ct, ok := contratos[cid]
				if !ok {
					ct, err = c.GetContrato(cid)
					if err != nil {
						return Puxado{}, fmt.Errorf("contrato %s: %w", cid, err)
					}
					contratos[cid] = ct
				}
				folhaContratos[cid] = ct
			}

			var escola *Escola
			if id := strings.TrimSpace(folha.EscolaID); id != "" {
				esc, ok := escolas[id]
				if !ok {
					esc, err = c.GetEscola(id)
					if err != nil {
						return Puxado{}, fmt.Errorf("escola %s: %w", id, err)
					}
					escolas[id] = esc
				}
				escola = &esc
			}

			var imp *ImportacaoEscola
			inep := strings.TrimSpace(folha.INEP)
			if EscolaConectada(escola) && inep != "" {
				if !impDone[inep] {
					rows, _, err := c.ListImportacoesEscola(DefaultPageSize, 0, ConstraintsINEP(inep))
					if err != nil {
						return Puxado{}, fmt.Errorf("importação_escola %s: %w", inep, err)
					}
					imps[inep] = ImportacaoComRelatorio(rows)
					impDone[inep] = true
				}
				imp = imps[inep]
			}

			item, skip := ProgramadoDaFolha(osp, folha, escola, folhaContratos, imp)
			if skip != "" {
				out.Skips = append(out.Skips, PuxarSkip{OSPID: osp.ID, FolhaID: folha.ID, INEP: inep, Motivo: skip})
				continue
			}
			item.Origem = OrigemDaBase(c.BaseURL)
			out.Itens = append(out.Itens, item)
		}
	}
	return out, nil
}

func (c *Client) listOSPsDoMes(mes time.Time) ([]OSP, error) {
	cons := ConstraintsOSPMes(mes)
	var all []OSP
	cursor := 0
	for {
		rows, page, err := c.ListOSPs(DefaultPageSize, cursor, cons)
		if err != nil {
			return nil, err
		}
		all = append(all, rows...)
		if len(rows) == 0 || page.Remaining == 0 {
			break
		}
		cursor += len(rows)
	}
	return all, nil
}
