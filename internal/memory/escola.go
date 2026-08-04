package memory

import (
	"errors"

	"github.com/wellington/oce_processamento/internal/domain"
)

// EscolaStore is an in-memory EscolaStore.
type EscolaStore struct {
	byINEP        map[string]domain.SituacaoOCE
	failRemaining int
}

func NewEscolaStore() *EscolaStore {
	return &EscolaStore{byINEP: make(map[string]domain.SituacaoOCE)}
}

func (s *EscolaStore) Seed(inep string, situacao domain.SituacaoOCE) {
	s.byINEP[inep] = situacao
}

func (s *EscolaStore) Get(inep string) (domain.SituacaoOCE, bool) {
	v, ok := s.byINEP[inep]
	return v, ok
}

// FailNext makes the next n ApplyBatch calls fail (for retry tests).
func (s *EscolaStore) FailNext(n int) {
	s.failRemaining = n
}

// UpdateSituacaoOCE updates existing Escola only; missing INEP is a no-op.
func (s *EscolaStore) UpdateSituacaoOCE(inep string, situacao domain.SituacaoOCE) {
	if _, ok := s.byINEP[inep]; !ok {
		return
	}
	s.byINEP[inep] = situacao
}

// ApplyBatch applies a batch of Situação OCE updates. Missing INEP is a no-op.
func (s *EscolaStore) ApplyBatch(items []domain.ItemLote) error {
	if s.failRemaining > 0 {
		s.failRemaining--
		return errors.New("falha transitória no batch")
	}
	for _, item := range items {
		s.UpdateSituacaoOCE(item.INEP, item.Situacao)
	}
	return nil
}
