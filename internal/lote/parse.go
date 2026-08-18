package lote

import (
	"encoding/csv"
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

// ParseCSV parses a Carga de Planejamento CSV (comma or semicolon).
// Requires the six expected headers; invalid lines are skipped;
// last Chave da Linha de Planejamento wins.
func ParseCSV(r io.Reader) ([]domain.ItemCarga, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	text := string(data)
	text = strings.TrimPrefix(text, "\ufeff")
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("csv vazio")
	}

	delim := detectDelimiter(text)
	cr := csv.NewReader(strings.NewReader(text))
	cr.Comma = delim
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("csv não parseável: %w", err)
	}
	idx, err := mapHeader(header)
	if err != nil {
		return nil, err
	}

	byChave := make(map[string]domain.ItemCarga)
	ordem := make([]string, 0)
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("csv não parseável: %w", err)
		}
		if len(rec) > len(header) {
			continue
		}
		item, ok := parseRow(rec, idx)
		if !ok {
			continue
		}
		k := chave(item)
		if _, seen := byChave[k]; !seen {
			ordem = append(ordem, k)
		}
		byChave[k] = item
	}
	if len(ordem) == 0 {
		return nil, fmt.Errorf("csv sem linhas válidas")
	}
	items := make([]domain.ItemCarga, 0, len(ordem))
	for _, k := range ordem {
		items = append(items, byChave[k])
	}
	return items, nil
}

func parseRow(rec []string, idx colIdx) (domain.ItemCarga, bool) {
	dataStr := cell(rec, idx.data)
	fase := cell(rec, idx.fase)
	regional := cell(rec, idx.regional)
	cnpj := cell(rec, idx.cnpj)
	qtdStr := cell(rec, idx.quantidade)
	if dataStr == "" || fase == "" || regional == "" || cnpj == "" || qtdStr == "" {
		return domain.ItemCarga{}, false
	}
	data, err := time.Parse("02/01/2006", dataStr)
	if err != nil {
		return domain.ItemCarga{}, false
	}
	qtd, err := strconv.Atoi(qtdStr)
	if err != nil || qtd < 0 {
		return domain.ItemCarga{}, false
	}
	return domain.ItemCarga{
		Tipo:           domain.TipoPlanejado,
		Data:           data.UTC(),
		Fase:           fase,
		Regional:       regional,
		RegionalNome:   regionalNomes[regional],
		FornecedorNome: cell(rec, idx.fornecedor),
		FornecedorCNPJ: cnpj,
		Quantidade:     qtd,
	}, true
}

func chave(item domain.ItemCarga) string {
	return item.Data.Format("2006-01-02") + "|" + item.Fase + "|" + item.Regional + "|" + item.FornecedorCNPJ
}

func cell(rec []string, i int) string {
	if i < 0 || i >= len(rec) {
		return ""
	}
	return strings.TrimSpace(rec[i])
}

func detectDelimiter(text string) rune {
	firstLine := text
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		firstLine = text[:i]
	}
	if strings.Count(firstLine, ";") > strings.Count(firstLine, ",") {
		return ';'
	}
	return ','
}

type colIdx struct {
	data, fase, regional, fornecedor, cnpj, quantidade int
}

func mapHeader(header []string) (colIdx, error) {
	idx := colIdx{data: -1, fase: -1, regional: -1, fornecedor: -1, cnpj: -1, quantidade: -1}
	for i, h := range header {
		switch strings.TrimSpace(strings.ToLower(h)) {
		case "data":
			idx.data = i
		case "fase":
			idx.fase = i
		case "regional":
			idx.regional = i
		case "fornecedor":
			idx.fornecedor = i
		case "cnpj":
			idx.cnpj = i
		case "quantidade":
			idx.quantidade = i
		}
	}
	if idx.data < 0 || idx.fase < 0 || idx.regional < 0 || idx.fornecedor < 0 || idx.cnpj < 0 || idx.quantidade < 0 {
		return colIdx{}, fmt.Errorf("csv sem header esperado")
	}
	return idx, nil
}
