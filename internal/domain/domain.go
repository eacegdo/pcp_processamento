package domain

import "time"

const TipoPlanejado = "planejado"
const TipoProgramado = "programado"

const OrigemVersionTest = "version-test"
const OrigemLive = "live"

// RegistroPCP is one line in the PCP collection.
type RegistroPCP struct {
	Tipo           string
	Data           time.Time
	Fase           string
	Regional       string
	RegionalNome   string
	UF             string
	INEP           string
	FornecedorNome string
	FornecedorCNPJ string
	Quantidade     int
	Provisoria     *bool
	Origem         string
}

// ItemCarga is one line to apply: Planejado (Carga de Planejamento) or Programado (JSON do Bubble).
type ItemCarga struct {
	Tipo           string
	Data           time.Time
	Fase           string
	Regional       string
	RegionalNome   string
	UF             string
	INEP           string
	FornecedorNome string
	FornecedorCNPJ string
	Quantidade     int
	Provisoria     *bool
	Origem         string
}

// Job is an Aplicação da Carga execution with observable progress.
type Job struct {
	ID           string
	Tipo         string
	Status       string
	Total        int
	Processadas  int
	Restantes    int
	FileName     string
	ErrorMessage string
	Items        []ItemCarga
}

// JobStore persists Jobs de Aplicação and supports FIFO claim.
type JobStore interface {
	Create(total int, tipo string, fileName string, items []ItemCarga) (Job, error)
	Get(id string) (Job, bool)
	Running() (Job, bool)
	ClaimNext() (Job, bool)
	MarkProgress(id string, processadas int) error
	MarkSuccess(id string) error
	MarkFailed(id string, errorMessage string) error
}

// PcpStore applies Planejado batches to the Registro PCP collection.
type PcpStore interface {
	ApplyBatch(items []ItemCarga) error
}
