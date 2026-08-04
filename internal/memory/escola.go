package memory

type SituacaoOCE struct {
	TipoAcesso string
	Status     string
	Pendencia  string
}

type EscolaStore struct {
	byINEP map[string]SituacaoOCE
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

// UpdateSituacaoOCE updates existing Escola only; missing INEP is a no-op.
func (s *EscolaStore) UpdateSituacaoOCE(inep string, situacao SituacaoOCE) {
	if _, ok := s.byINEP[inep]; !ok {
		return
	}
	s.byINEP[inep] = situacao
}
