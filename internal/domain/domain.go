package domain

// SituacaoOCE is the OCE state of an Escola.
type SituacaoOCE struct {
	TipoAcesso string
	Status     string
	Pendencia  string
}

// ItemLote is one Escola's Situação OCE to apply in a Lote OCE.
type ItemLote struct {
	INEP     string
	Situacao SituacaoOCE
}

// Job is an Aplicação de Lote execution with observable progress.
type Job struct {
	ID           string
	Status       string
	Total        int
	Processadas  int
	Restantes    int
	FileName     string
	ErrorMessage string
	Items        []ItemLote
}

// JobStore persists Jobs de Aplicação and supports FIFO claim.
type JobStore interface {
	Create(total int, fileName string, items []ItemLote) (Job, error)
	Get(id string) (Job, bool)
	Running() (Job, bool)
	ClaimNext() (Job, bool)
	MarkProgress(id string, processadas int) error
	MarkSuccess(id string) error
	MarkFailed(id string, errorMessage string) error
}

// EscolaStore updates Situação OCE on existing Escolas only.
type EscolaStore interface {
	ApplyBatch(items []ItemLote) error
}
