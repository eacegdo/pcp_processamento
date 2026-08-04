package worker

import "github.com/wellington/oce_processamento/internal/domain"

type Config struct {
	BatchSize  int
	MaxRetries int
}

type Worker struct {
	jobs       domain.JobStore
	escolas    domain.EscolaStore
	batchSize  int
	maxRetries int
}

func New(jobs domain.JobStore, escolas domain.EscolaStore, cfg Config) *Worker {
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 200
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	return &Worker{jobs: jobs, escolas: escolas, batchSize: batchSize, maxRetries: maxRetries}
}

// ProcessNext claims the next queued Job (or continues a running one)
// and applies a single batch. Returns false when there is no work.
func (w *Worker) ProcessNext() bool {
	job, ok := w.jobs.Running()
	if !ok {
		job, ok = w.jobs.ClaimNext()
		if !ok {
			return false
		}
	}

	start := job.Processadas
	if start >= job.Total {
		w.finishSuccess(job.ID)
		return true
	}
	end := start + w.batchSize
	if end > job.Total {
		end = job.Total
	}
	batch := job.Items[start:end]

	var err error
	for attempt := 0; attempt < w.maxRetries; attempt++ {
		err = w.escolas.ApplyBatch(batch)
		if err == nil {
			break
		}
	}
	if err != nil {
		w.finishFailed(job.ID, err.Error())
		return true
	}

	if err := w.jobs.MarkProgress(job.ID, end); err != nil {
		w.finishFailed(job.ID, err.Error())
		return true
	}
	if end >= job.Total {
		w.finishSuccess(job.ID)
	}
	return true
}

func (w *Worker) finishSuccess(id string) {
	if err := w.jobs.MarkSuccess(id); err != nil {
		w.finishFailed(id, err.Error())
	}
}

func (w *Worker) finishFailed(id, msg string) {
	for attempt := 0; attempt < w.maxRetries; attempt++ {
		if err := w.jobs.MarkFailed(id, msg); err == nil {
			return
		}
	}
}
