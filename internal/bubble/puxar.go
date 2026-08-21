package bubble

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/wellington/pcp_processamento/internal/domain"
)

const puxarWorkers = 12

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

	frIDs := make([]string, 0)
	for _, osp := range osps {
		frIDs = append(frIDs, osp.FRs...)
	}
	log.Printf("puxar: %d OSPs, %d folhas; buscando em lote", len(osps), len(uniqueNonEmpty(frIDs)))

	folhas, err := c.folhasPorIDs(frIDs)
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
	contratos, err := c.contratosPorIDs(contratoIDs)
	if err != nil {
		return Puxado{}, err
	}
	escolas, err := c.escolasPorIDs(escolaIDs)
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
	imps, err := c.importacoesPorINEPs(ineps)
	if err != nil {
		return Puxado{}, err
	}

	origem := OrigemDaBase(c.BaseURL)
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
			item.Origem = origem
			out.Itens = append(out.Itens, item)
		}
	}
	return out, nil
}

func (c *Client) listOSPsDoMes(mes time.Time) ([]OSP, error) {
	cons := ConstraintsOSPMes(mes)
	first, page, err := c.ListOSPs(DefaultPageSize, 0, cons)
	if err != nil {
		return nil, err
	}
	if page.Remaining == 0 || len(first) == 0 {
		return first, nil
	}

	pageSize := len(first)
	total := pageSize + page.Remaining
	nPages := (total + pageSize - 1) / pageSize
	pages := make([][]OSP, nPages)
	pages[0] = first

	err = parallelDo(nPages-1, puxarWorkers, func(i int) error {
		cursor := (i + 1) * pageSize
		rows, _, err := c.ListOSPs(pageSize, cursor, cons)
		if err != nil {
			return err
		}
		pages[i+1] = rows
		return nil
	})
	if err != nil {
		return nil, err
	}

	all := make([]OSP, 0, total)
	for _, p := range pages {
		all = append(all, p...)
	}
	return all, nil
}

func (c *Client) folhasPorIDs(ids []string) (map[string]FolhaOSP, error) {
	return loadByIDs(ids,
		func(part []string) ([]FolhaOSP, error) {
			rows, _, err := c.ListFolhasOSPConstrained(DefaultPageSize, 0, ConstraintsIDs(part))
			return rows, err
		},
		c.GetFolhaOSP,
		func(row FolhaOSP) string { return row.ID },
	)
}

func (c *Client) contratosPorIDs(ids []string) (map[string]ContratoInstalacao, error) {
	return loadByIDs(ids,
		func(part []string) ([]ContratoInstalacao, error) {
			rows, _, err := c.ListContratos(DefaultPageSize, 0, ConstraintsIDs(part))
			return rows, err
		},
		c.GetContrato,
		func(row ContratoInstalacao) string { return row.ID },
	)
}

func (c *Client) escolasPorIDs(ids []string) (map[string]Escola, error) {
	return loadByIDs(ids,
		func(part []string) ([]Escola, error) {
			rows, _, err := c.ListEscolas(DefaultPageSize, 0, ConstraintsIDs(part))
			return rows, err
		},
		c.GetEscola,
		func(row Escola) string { return row.ID },
	)
}

func (c *Client) importacoesPorINEPs(ineps []string) (map[string]*ImportacaoEscola, error) {
	ineps = uniqueNonEmpty(ineps)
	out := make(map[string]*ImportacaoEscola, len(ineps))
	if len(ineps) == 0 {
		return out, nil
	}

	grouped := make(map[string][]ImportacaoEscola, len(ineps))
	parts := chunks(ineps, DefaultPageSize)
	var mu sync.Mutex
	err := parallelDo(len(parts), puxarWorkers, func(i int) error {
		cons := ConstraintsINEPs(parts[i])
		cursor := 0
		for {
			rows, page, err := c.ListImportacoesEscola(DefaultPageSize, cursor, cons)
			if err != nil {
				return err
			}
			mu.Lock()
			for _, row := range rows {
				k := strings.TrimSpace(row.INEP)
				if k == "" {
					continue
				}
				grouped[k] = append(grouped[k], row)
			}
			mu.Unlock()
			if len(rows) == 0 || page.Remaining == 0 {
				break
			}
			cursor += len(rows)
		}
		return nil
	})
	if err != nil {
		return c.importacoesPorINEPUmAUm(ineps)
	}
	for _, inep := range ineps {
		out[inep] = ImportacaoComRelatorio(grouped[inep])
	}
	return out, nil
}

func (c *Client) importacoesPorINEPUmAUm(ineps []string) (map[string]*ImportacaoEscola, error) {
	out := make(map[string]*ImportacaoEscola, len(ineps))
	var mu sync.Mutex
	err := parallelDo(len(ineps), puxarWorkers, func(i int) error {
		rows, _, err := c.ListImportacoesEscola(DefaultPageSize, 0, ConstraintsINEP(ineps[i]))
		if err != nil {
			return fmt.Errorf("importação_escola %s: %w", ineps[i], err)
		}
		mu.Lock()
		out[ineps[i]] = ImportacaoComRelatorio(rows)
		mu.Unlock()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func loadByIDs[T any](
	ids []string,
	listChunk func([]string) ([]T, error),
	getOne func(string) (T, error),
	idOf func(T) string,
) (map[string]T, error) {
	ids = uniqueNonEmpty(ids)
	out := make(map[string]T, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	parts := chunks(ids, DefaultPageSize)
	var mu sync.Mutex
	listErr := parallelDo(len(parts), puxarWorkers, func(i int) error {
		rows, err := listChunk(parts[i])
		if err != nil {
			return err
		}
		mu.Lock()
		for _, row := range rows {
			out[idOf(row)] = row
		}
		mu.Unlock()
		return nil
	})

	var missing []string
	if listErr != nil {
		missing = ids
		out = make(map[string]T, len(ids))
	} else {
		for _, id := range ids {
			if _, ok := out[id]; !ok {
				missing = append(missing, id)
			}
		}
	}
	if len(missing) == 0 {
		return out, nil
	}

	err := parallelDo(len(missing), puxarWorkers, func(i int) error {
		row, err := getOne(missing[i])
		if err != nil {
			return fmt.Errorf("%s: %w", missing[i], err)
		}
		mu.Lock()
		out[idOf(row)] = row
		mu.Unlock()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func uniqueNonEmpty(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func chunks(ids []string, n int) [][]string {
	if n <= 0 {
		n = DefaultPageSize
	}
	var out [][]string
	for i := 0; i < len(ids); i += n {
		end := i + n
		if end > len(ids) {
			end = len(ids)
		}
		out = append(out, ids[i:end])
	}
	return out
}

func parallelDo(n, workers int, fn func(i int) error) error {
	if n <= 0 {
		return nil
	}
	if workers < 1 {
		workers = 1
	}
	sem := make(chan struct{}, workers)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := fn(i); err != nil {
				select {
				case errCh <- err:
				default:
				}
			}
		}(i)
	}
	wg.Wait()
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}
