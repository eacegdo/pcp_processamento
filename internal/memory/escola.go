package memory

import "errors"

type SituacaoOCE struct {
	TipoAcesso string
	Status     string
	Pendencia  string
}

type EscolaStore struct {
	byINEP        map[string]SituacaoOCE
	failRemaining int
}

func NewEscolaStore() *EscolaStore {
	return &EscolaStore{byINEP: make(map[string]SituacaoOCE)}
}

func (s *EscolaStore) Seed(inep string, situacao SituacaoOCE) {
	s.byINEP[inep] = situacao
}

func (s *EscolaStore) Get(inep string) (SituacaoOCE, bool) {
	v, ok := s.byINEP[inep]
	return v, ok
}

// FailNext makes the next n ApplyBatch calls fail (for retry tests).
func (s *EscolaStore) FailNext(n int) {
	s.failRemaining = n
}

// UpdateSituacaoOCE updates existing Escola only; missing INEP is a no-op.
func (s *EscolaStore) UpdateSituacaoOCE(inep string, situacao SituacaoOCE) {
	if _, ok := s.byINEP[inep]; !ok {
		return
	}
	s.byINEP[inep] = situacao
}

// ApplyBatch applies a batch of Situação OCE updates. Missing INEP is a no-op.
func (s *EscolaStore) ApplyBatch(items []ItemLote) error {
	if s.failRemaining > 0 {
		s.failRemaining--
		return errors.New("falha transitória no batch")
	}
	for _, item := range items {
		s.UpdateSituacaoOCE(item.INEP, item.Situacao)
	}
	return nil
}
