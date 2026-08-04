package worker

import "github.com/wellington/oce_processamento/internal/memory"

type Worker struct {
	jobs    *memory.JobStore
	escolas *memory.EscolaStore
}

func New(jobs *memory.JobStore, escolas *memory.EscolaStore) *Worker {
	return &Worker{jobs: jobs, escolas: escolas}
}

// ProcessNext claims the next queued Job and applies its Lote OCE rows.
func (w *Worker) ProcessNext() bool {
	job, ok := w.jobs.ClaimNext()
	if !ok {
		return false
	}
	for i, item := range job.Items {
		w.escolas.UpdateSituacaoOCE(item.INEP, item.Situacao)
		w.jobs.MarkProgress(job.ID, i+1)
	}
	w.jobs.MarkSuccess(job.ID)
	return true
}
