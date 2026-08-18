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
	UF             string `json:"uf,omitempty"`
	INEP           string `json:"inep,omitempty"`
	FornecedorNome string `json:"fornecedor_nome"`
	FornecedorCNPJ string `json:"fornecedor_cnpj"`
	Quantidade     int    `json:"quantidade"`
	Provisoria     *bool  `json:"provisoria,omitempty"`
}

// ApplyBatch applies the whole batch in one RPC call.
func (s *PcpStore) ApplyBatch(items []domain.ItemCarga) error {
	if len(items) == 0 {
		return nil
	}
	path := "/rpc/aplicar_carga_planejamento"
	if items[0].Tipo == domain.TipoProgramado {
		path = "/rpc/aplicar_programado"
	}

	rows := make([]rpcItem, 0, len(items))
	for _, item := range items {
		rows = append(rows, rpcItem{
			Data:           item.Data.Format("2006-01-02"),
			Fase:           item.Fase,
			Regional:       item.Regional,
			RegionalNome:   item.RegionalNome,
			UF:             item.UF,
			INEP:           item.INEP,
			FornecedorNome: item.FornecedorNome,
			FornecedorCNPJ: item.FornecedorCNPJ,
			Quantidade:     item.Quantidade,
			Provisoria:     item.Provisoria,
		})
	}
	body, err := json.Marshal(map[string]any{"itens": rows})
	if err != nil {
		return err
	}
	resp, err := s.client.do(http.MethodPost, path, bytes.NewReader(body), "")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
