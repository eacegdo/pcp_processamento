package lote

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/wellington/oce_processamento/internal/domain"
)

// ParseCSV parses a Lote OCE CSV (comma or semicolon).
// Requires the four expected headers; empty Situação OCE fields are kept;
// rows without INEP are skipped; last INEP wins.
func ParseCSV(r io.Reader) ([]domain.ItemLote, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	text := string(data)
	text = strings.TrimPrefix(text, "\ufeff") // Excel UTF-8 BOM
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("csv vazio")
	}

	delim := detectDelimiter(text)
	cr := csv.NewReader(strings.NewReader(text))
	cr.Comma = delim
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1 // allow short rows; incomplete lines are skipped

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("csv não parseável: %w", err)
	}
	idx, err := mapHeader(header)
	if err != nil {
		return nil, err
	}

	byINEP := make(map[string]domain.ItemLote)
	primeiraOrdemINEP := make([]string, 0)
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("csv não parseável: %w", err)
		}
		inep := cell(rec, idx.inep)
		if inep == "" {
			continue // sem INEP não há Escola a atualizar
		}
		tipo := cell(rec, idx.tipo)
		status := cell(rec, idx.status)
		pend := cell(rec, idx.pendencia)
		item := domain.ItemLote{
			INEP: inep,
			Situacao: domain.SituacaoOCE{
				TipoAcesso: tipo,
				Status:     status,
				Pendencia:  pend,
			},
		}
		if _, seen := byINEP[inep]; !seen {
			primeiraOrdemINEP = append(primeiraOrdemINEP, inep)
		}
		byINEP[inep] = item // last occurrence wins
	}
	if len(primeiraOrdemINEP) == 0 {
		return nil, fmt.Errorf("csv sem linhas válidas")
	}
	items := make([]domain.ItemLote, 0, len(primeiraOrdemINEP))
	for _, inep := range primeiraOrdemINEP {
		items = append(items, byINEP[inep])
	}
	return items, nil
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
	inep, tipo, status, pendencia int
}

func mapHeader(header []string) (colIdx, error) {
	idx := colIdx{inep: -1, tipo: -1, status: -1, pendencia: -1}
	for i, h := range header {
		switch strings.TrimSpace(strings.ToLower(h)) {
		case "inep":
			idx.inep = i
		case "oce_tipo_acesso":
			idx.tipo = i
		case "oce_status_final":
			idx.status = i
		case "oce_pendencia":
			idx.pendencia = i
		}
	}
	if idx.inep < 0 || idx.tipo < 0 || idx.status < 0 || idx.pendencia < 0 {
		return colIdx{}, fmt.Errorf("csv sem header esperado")
	}
	return idx, nil
}
