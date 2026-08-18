package supabase

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/wellington/pcp_processamento/internal/domain"
)

type PcpStore struct {
	client *Client
}

func NewPcpStore(supabaseURL, serviceRoleKey string, httpClient *http.Client) *PcpStore {
	return &PcpStore{client: NewClient(supabaseURL, serviceRoleKey, httpClient)}
}

type rpcItem struct {
	Data           string `json:"data"`
	Fase           string `json:"fase"`
	Regional       string `json:"regional"`
	RegionalNome   string `json:"regional_nome"`
	FornecedorNome string `json:"fornecedor_nome"`
	FornecedorCNPJ string `json:"fornecedor_cnpj"`
	Quantidade     int    `json:"quantidade"`
}

// ApplyBatch applies the whole batch in one RPC call (upsert Planejado by chave).
func (s *PcpStore) ApplyBatch(items []domain.ItemCarga) error {
	if len(items) == 0 {
		return nil
	}

	rows := make([]rpcItem, 0, len(items))
	for _, item := range items {
		rows = append(rows, rpcItem{
			Data:           item.Data.Format("2006-01-02"),
			Fase:           item.Fase,
			Regional:       item.Regional,
			RegionalNome:   item.RegionalNome,
			FornecedorNome: item.FornecedorNome,
			FornecedorCNPJ: item.FornecedorCNPJ,
			Quantidade:     item.Quantidade,
		})
	}
	body, err := json.Marshal(map[string]any{"itens": rows})
	if err != nil {
		return err
	}
	resp, err := s.client.do(http.MethodPost, "/rpc/aplicar_carga_planejamento", bytes.NewReader(body), "")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
