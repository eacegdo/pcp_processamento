package programado

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/wellington/pcp_processamento/internal/domain"
)

var regionalNomes = map[string]string{
	"NO":    "Norte",
	"NE-I":  "Nordeste I",
	"NE-II": "Nordeste II",
	"SUSE":  "Sudeste/Centro-Sul",
	"COSE":  "Centro-Oeste/Minas",
}

type rawItem struct {
	Data           string          `json:"data"`
	Fase           string          `json:"fase"`
	Regional       string          `json:"regional"`
	UF             string          `json:"uf"`
	INEP           json.RawMessage `json:"inep"`
	FornecedorNome string          `json:"fornecedor_nome"`
	FornecedorCNPJ string          `json:"fornecedor_cnpj"`
	Quantidade     *int            `json:"quantidade"`
	Provisoria     *bool           `json:"provisoria"`
	Origem         string          `json:"origem"`
}

// ParseJSON reads a JSON array of Programado objects, or {"itens":[...]}.
// Invalid objects are skipped; last Data+INEP wins.
// The civil month of the first valid item is the Espelho do Mês; other months are dropped.
func ParseJSON(r io.Reader) ([]domain.ItemCarga, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, fmt.Errorf("json vazio")
	}

	var items []rawItem
	switch raw[0] {
	case '[':
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("json inválido: %w", err)
		}
	case '{':
		var wrap struct {
			Itens []rawItem `json:"itens"`
		}
		if err := json.Unmarshal(raw, &wrap); err != nil {
			return nil, fmt.Errorf("json inválido: %w", err)
		}
		items = wrap.Itens
	default:
		return nil, fmt.Errorf("json inválido")
	}

	byChave := make(map[string]domain.ItemCarga)
	ordem := make([]string, 0)
	var mes time.Time
	temMes := false
	for _, rawItem := range items {
		item, ok := toItem(rawItem)
		if !ok {
			continue
		}
		if !temMes {
			mes = time.Date(item.Data.Year(), item.Data.Month(), 1, 0, 0, 0, 0, time.UTC)
			temMes = true
		} else if item.Data.Year() != mes.Year() || item.Data.Month() != mes.Month() {
			continue
		}
		k := item.Data.Format("2006-01-02") + "|" + item.INEP
		if _, seen := byChave[k]; !seen {
			ordem = append(ordem, k)
		}
		byChave[k] = item
	}
	if len(ordem) == 0 {
		return nil, fmt.Errorf("json sem objetos válidos")
	}
	out := make([]domain.ItemCarga, 0, len(ordem))
	for _, k := range ordem {
		out = append(out, byChave[k])
	}
	return out, nil
}

func toItem(r rawItem) (domain.ItemCarga, bool) {
	inep := parseINEP(r.INEP)
	data, ok := parseData(strings.TrimSpace(r.Data))
	fase := strings.TrimSpace(r.Fase)
	regional, regionalNome := resolveRegional(r.Regional)
	if inep == "" || !ok || fase == "" || regional == "" {
		return domain.ItemCarga{}, false
	}
	qtd := 1
	if r.Quantidade != nil {
		if *r.Quantidade < 0 {
			return domain.ItemCarga{}, false
		}
		qtd = *r.Quantidade
	}
	return domain.ItemCarga{
		Tipo:           domain.TipoProgramado,
		Data:           data,
		Fase:           fase,
		Regional:       regional,
		RegionalNome:   regionalNome,
		UF:             strings.TrimSpace(r.UF),
		INEP:           inep,
		FornecedorNome: strings.TrimSpace(r.FornecedorNome),
		FornecedorCNPJ: strings.TrimSpace(r.FornecedorCNPJ),
		Quantidade:     qtd,
		Provisoria:     r.Provisoria,
		Origem:         strings.TrimSpace(r.Origem),
	}, true
}

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

func parseINEP(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return ""
	}
	if strings.HasPrefix(s, `"`) {
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			return ""
		}
		return strings.TrimSpace(v)
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return strings.Trim(s, `"`)
	}
	return strconv.FormatInt(int64(n), 10)
}

func parseData(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		"02/01/2006",
		"2006-01-02",
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
		}
	}
	return time.Time{}, false
}
