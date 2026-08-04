package supabase

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/wellington/oce_processamento/internal/domain"
)

type EscolaStore struct {
	client *Client
}

func NewEscolaStore(supabaseURL, serviceRoleKey string, httpClient *http.Client) *EscolaStore {
	return &EscolaStore{client: NewClient(supabaseURL, serviceRoleKey, httpClient)}
}

// ApplyBatch PATCHes only oce_tipo_acesso, oce_status, oce_pendencia by inep.
// Missing INEP is a no-op (PostgREST updates zero rows). Never inserts.
func (s *EscolaStore) ApplyBatch(items []domain.ItemLote) error {
	for _, item := range items {
		payload := map[string]string{
			"oce_tipo_acesso": item.Situacao.TipoAcesso,
			"oce_status":      item.Situacao.Status,
			"oce_pendencia":   item.Situacao.Pendencia,
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		q := url.Values{}
		q.Set("inep", "eq."+item.INEP)
		resp, err := s.client.do(http.MethodPatch, "/escola?"+q.Encode(), bytes.NewReader(body), "return=minimal")
		if err != nil {
			return err
		}
		resp.Body.Close()
	}
	return nil
}
