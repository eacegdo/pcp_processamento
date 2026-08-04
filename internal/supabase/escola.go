package supabase

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/wellington/oce_processamento/internal/domain"
)

type EscolaStore struct {
	client *Client
}

func NewEscolaStore(supabaseURL, serviceRoleKey string, httpClient *http.Client) *EscolaStore {
	return &EscolaStore{client: NewClient(supabaseURL, serviceRoleKey, httpClient)}
}

// ApplyBatch applies the whole batch in one RPC call (UPDATE only; missing INEP is no-op).
func (s *EscolaStore) ApplyBatch(items []domain.ItemLote) error {
	if len(items) == 0 {
		return nil
	}

	rows := make([]map[string]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, map[string]string{
			"inep":            item.INEP,
			"oce_tipo_acesso": item.Situacao.TipoAcesso,
			"oce_status":      item.Situacao.Status,
			"oce_pendencia":   item.Situacao.Pendencia,
		})
	}
	body, err := json.Marshal(map[string]any{"itens": rows})
	if err != nil {
		return err
	}
	resp, err := s.client.do(http.MethodPost, "/rpc/aplicar_situacao_oce_lote", bytes.NewReader(body), "")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
