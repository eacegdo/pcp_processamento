package worker

import "github.com/wellington/oce_processamento/internal/memory"

type Config struct {
	BatchSize  int
	MaxRetries int
}

type Worker struct {
	jobs       *memory.JobStore
	escolas    *memory.EscolaStore
	batchSize  int
	maxRetries int
}

func New(jobs *memory.JobStore, escolas *memory.EscolaStore, cfg Config) *Worker {
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
		w.jobs.MarkSuccess(job.ID)
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
		w.jobs.MarkFailed(job.ID, err.Error())
		return true
	}

	w.jobs.MarkProgress(job.ID, end)
	if end >= job.Total {
		w.jobs.MarkSuccess(job.ID)
	}
	return true
}
