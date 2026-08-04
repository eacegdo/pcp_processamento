package lote

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/wellington/oce_processamento/internal/memory"
)

// ParseCSV parses a minimal Lote OCE CSV (comma or semicolon).
// Requires the four expected headers. Empty fields are skipped (issue 02 will harden further).
func ParseCSV(r io.Reader) ([]memory.ItemLote, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	text := string(data)
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("csv vazio")
	}

	delim := detectDelimiter(text)
	cr := csv.NewReader(strings.NewReader(text))
	cr.Comma = delim
	cr.TrimLeadingSpace = true

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("csv não parseável: %w", err)
	}
	idx, err := mapHeader(header)
	if err != nil {
		return nil, err
	}

	var items []memory.ItemLote
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("csv não parseável: %w", err)
		}
		inep := strings.TrimSpace(rec[idx.inep])
		tipo := strings.TrimSpace(rec[idx.tipo])
		status := strings.TrimSpace(rec[idx.status])
		pend := strings.TrimSpace(rec[idx.pendencia])
		if inep == "" || tipo == "" || status == "" || pend == "" {
			continue
		}
		items = append(items, memory.ItemLote{
			INEP: inep,
			Situacao: memory.SituacaoOCE{
				TipoAcesso: tipo,
				Status:     status,
				Pendencia:  pend,
			},
		})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("csv sem linhas válidas")
	}
	return items, nil
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
