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

func chavePlanejado(data time.Time, fase, regional, cnpj string) string {
	d := time.Date(data.Year(), data.Month(), data.Day(), 0, 0, 0, 0, time.UTC)
	return "p|" + d.Format("2006-01-02") + "|" + fase + "|" + regional + "|" + cnpj
}

func chaveProgramado(data time.Time, inep string) string {
	d := time.Date(data.Year(), data.Month(), data.Day(), 0, 0, 0, 0, time.UTC)
	return "g|" + d.Format("2006-01-02") + "|" + inep
}

func (s *PcpStore) Get(data time.Time, fase, regional, cnpj string) (domain.RegistroPCP, bool) {
	v, ok := s.byChave[chavePlanejado(data, fase, regional, cnpj)]
	return v, ok
}

func (s *PcpStore) GetProgramado(data time.Time, inep string) (domain.RegistroPCP, bool) {
	v, ok := s.byChave[chaveProgramado(data, inep)]
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
	if len(items) > 0 && items[0].Tipo == domain.TipoProgramado {
		s.applyEspelhoProgramado(items)
		return nil
	}
	for _, item := range items {
		if item.Quantidade < 0 {
			continue
		}
		k := chavePlanejado(item.Data, item.Fase, item.Regional, item.FornecedorCNPJ)
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

func (s *PcpStore) applyEspelhoProgramado(items []domain.ItemCarga) {
	mes := time.Date(items[0].Data.Year(), items[0].Data.Month(), 1, 0, 0, 0, 0, time.UTC)
	chavesRecebidas := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.Tipo != domain.TipoProgramado || item.Quantidade < 0 {
			continue
		}
		if item.Data.Year() != mes.Year() || item.Data.Month() != mes.Month() {
			continue
		}
		s.applyProgramado(item)
		chavesRecebidas[chaveProgramado(item.Data, item.INEP)] = struct{}{}
	}
	for k, rec := range s.byChave {
		if rec.Tipo != domain.TipoProgramado {
			continue
		}
		if rec.Data.Year() != mes.Year() || rec.Data.Month() != mes.Month() {
			continue
		}
		if _, ok := chavesRecebidas[k]; !ok {
			delete(s.byChave, k)
		}
	}
}

func (s *PcpStore) applyProgramado(item domain.ItemCarga) {
	k := chaveProgramado(item.Data, item.INEP)
	existing, ok := s.byChave[k]
	if ok {
		existing.Fase = item.Fase
		existing.Regional = item.Regional
		existing.RegionalNome = item.RegionalNome
		existing.UF = item.UF
		existing.FornecedorNome = item.FornecedorNome
		existing.FornecedorCNPJ = item.FornecedorCNPJ
		existing.Quantidade = item.Quantidade
		existing.Provisoria = item.Provisoria
		s.byChave[k] = existing
		return
	}
	if item.Quantidade == 0 {
		return
	}
	s.byChave[k] = domain.RegistroPCP{
		Tipo:           domain.TipoProgramado,
		Data:           time.Date(item.Data.Year(), item.Data.Month(), item.Data.Day(), 0, 0, 0, 0, time.UTC),
		Fase:           item.Fase,
		Regional:       item.Regional,
		RegionalNome:   item.RegionalNome,
		UF:             item.UF,
		INEP:           item.INEP,
		FornecedorNome: item.FornecedorNome,
		FornecedorCNPJ: item.FornecedorCNPJ,
		Quantidade:     item.Quantidade,
		Provisoria:     item.Provisoria,
	}
}
