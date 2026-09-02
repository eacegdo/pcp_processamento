package bubble

import (
	"encoding/json"
	"fmt"
	"sort"
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
	got, err := MontarMes(c, mes)
	if err != nil {
		return Puxado{}, err
	}
	origem := OrigemDaBase(c.BaseURL)
	for i := range got.Itens {
		got.Itens[i].Origem = origem
	}
	return got, nil
}

func (c *Client) OSPsDoMes(mes time.Time) ([]OSP, error) {
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

func (c *Client) FolhasPorIDs(ids []string) (map[string]FolhaOSP, error) {
	return loadByIDs(ids,
		func(part []string) ([]FolhaOSP, error) {
			rows, _, err := c.ListFolhasOSPConstrained(DefaultPageSize, 0, ConstraintsIDs(part))
			return rows, err
		},
		c.GetFolhaOSP,
		func(row FolhaOSP) string { return row.ID },
	)
}

func (c *Client) ContratosPorIDs(ids []string) (map[string]ContratoInstalacao, error) {
	return loadByIDs(ids,
		func(part []string) ([]ContratoInstalacao, error) {
			rows, _, err := c.ListContratos(DefaultPageSize, 0, ConstraintsIDs(part))
			return rows, err
		},
		c.GetContrato,
		func(row ContratoInstalacao) string { return row.ID },
	)
}

func (c *Client) EscolasPorIDs(ids []string) (map[string]Escola, error) {
	return loadByIDs(ids,
		func(part []string) ([]Escola, error) {
			rows, _, err := c.ListEscolas(DefaultPageSize, 0, ConstraintsIDs(part))
			return rows, err
		},
		c.GetEscola,
		func(row Escola) string { return row.ID },
	)
}

func (c *Client) ImportacoesPorINEPs(ineps []string) (map[string]*ImportacaoEscola, error) {
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

// OSPsPorIDs traz as OSPs desses IDs, em lote e paginado como as demais buscas.
func (c *Client) OSPsPorIDs(ids []string) (map[string]OSP, error) {
	return loadByIDs(ids,
		func(part []string) ([]OSP, error) {
			rows, _, err := c.ListOSPs(DefaultPageSize, 0, ConstraintsIDs(part))
			return rows, err
		},
		c.GetOSP,
		func(row OSP) string { return row.ID },
	)
}

// ImportacoesDoMes traz as importações de escola com data_relatorio dentro do mês civil.
func (c *Client) ImportacoesDoMes(mes time.Time) ([]ImportacaoEscola, error) {
	cons := ConstraintsImportacaoMes(mes)
	out := make([]ImportacaoEscola, 0)
	cursor := 0
	for {
		rows, page, err := c.ListImportacoesEscola(DefaultPageSize, cursor, cons)
		if err != nil {
			return nil, fmt.Errorf("importações do mês %s: %w", mes.Format("2006-01"), err)
		}
		out = append(out, rows...)
		if len(rows) == 0 || page.Remaining == 0 {
			return out, nil
		}
		cursor += len(rows)
	}
}

// FolhasPorINEPs traz as Folhas de Registro cujo INEP (o da folha) está na lista,
// em chunks paralelos e paginados, com o mesmo fallback de ImportacoesPorINEPs.
func (c *Client) FolhasPorINEPs(ineps []string) ([]FolhaOSP, error) {
	ineps = uniqueNonEmpty(ineps)
	if len(ineps) == 0 {
		return nil, nil
	}

	parts := chunks(ineps, DefaultPageSize)
	var mu sync.Mutex
	porID := make(map[string]FolhaOSP)
	err := parallelDo(len(parts), puxarWorkers, func(i int) error {
		return c.coletaFolhasPorINEPs(parts[i], &mu, porID)
	})
	if err != nil {
		porID = make(map[string]FolhaOSP)
		err = parallelDo(len(ineps), puxarWorkers, func(i int) error {
			if err := c.coletaFolhasPorINEPs(ineps[i:i+1], &mu, porID); err != nil {
				return fmt.Errorf("fr_osp INEP %s: %w", ineps[i], err)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	out := make([]FolhaOSP, 0, len(porID))
	for _, folha := range porID {
		out = append(out, folha)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (c *Client) coletaFolhasPorINEPs(ineps []string, mu *sync.Mutex, porID map[string]FolhaOSP) error {
	cons := ConstraintsFolhasINEPs(ineps)
	cursor := 0
	for {
		rows, page, err := c.ListFolhasOSPConstrained(DefaultPageSize, cursor, cons)
		if err != nil {
			return err
		}
		mu.Lock()
		for _, row := range rows {
			if id := strings.TrimSpace(row.ID); id != "" {
				porID[id] = row
			}
		}
		mu.Unlock()
		if len(rows) == 0 || page.Remaining == 0 {
			return nil
		}
		cursor += len(rows)
	}
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
