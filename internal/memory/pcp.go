package memory

import (
	"errors"
	"time"

	"github.com/wellington/pcp_processamento/internal/domain"
)

// PcpStore is an in-memory PcpStore.
type PcpStore struct {
	byChave       map[string]domain.RegistroPCP
	failRemaining int
}

func NewPcpStore() *PcpStore {
	return &PcpStore{byChave: make(map[string]domain.RegistroPCP)}
}

func chave(data time.Time, fase, regional, cnpj string) string {
	d := time.Date(data.Year(), data.Month(), data.Day(), 0, 0, 0, 0, time.UTC)
	return d.Format("2006-01-02") + "|" + fase + "|" + regional + "|" + cnpj
}

func (s *PcpStore) Get(data time.Time, fase, regional, cnpj string) (domain.RegistroPCP, bool) {
	v, ok := s.byChave[chave(data, fase, regional, cnpj)]
	return v, ok
}

func (s *PcpStore) Count() int {
	return len(s.byChave)
}

// FailNext makes the next n ApplyBatch calls fail (for retry tests).
func (s *PcpStore) FailNext(n int) {
	s.failRemaining = n
}

func (s *PcpStore) ApplyBatch(items []domain.ItemCarga) error {
	if s.failRemaining > 0 {
		s.failRemaining--
		return errors.New("falha transitória no batch")
	}
	for _, item := range items {
		if item.Quantidade < 0 {
			continue
		}
		k := chave(item.Data, item.Fase, item.Regional, item.FornecedorCNPJ)
		existing, ok := s.byChave[k]
		if ok {
			existing.Quantidade = item.Quantidade
			existing.FornecedorNome = item.FornecedorNome
			existing.RegionalNome = item.RegionalNome
			s.byChave[k] = existing
			continue
		}
		if item.Quantidade == 0 {
			continue
		}
		s.byChave[k] = domain.RegistroPCP{
			Tipo:           domain.TipoPlanejado,
			Data:           time.Date(item.Data.Year(), item.Data.Month(), item.Data.Day(), 0, 0, 0, 0, time.UTC),
			Fase:           item.Fase,
			Regional:       item.Regional,
			RegionalNome:   item.RegionalNome,
			FornecedorNome: item.FornecedorNome,
			FornecedorCNPJ: item.FornecedorCNPJ,
			Quantidade:     item.Quantidade,
		}
	}
	return nil
}
