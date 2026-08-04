package memory

import (
	"sync"

	"github.com/google/uuid"
	"github.com/wellington/oce_processamento/internal/domain"
)

// JobStore is an in-memory JobStore.
type JobStore struct {
	mu        sync.Mutex
	byID      map[string]*domain.Job
	fifo      []string
	runningID string
}

func NewJobStore() *JobStore {
	return &JobStore{byID: make(map[string]*domain.Job)}
}

func (s *JobStore) Create(total int, fileName string, items []domain.ItemLote) (domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.NewString()
	job := &domain.Job{
		ID:        id,
		Status:    "queued",
		Total:     total,
		Restantes: total,
		FileName:  fileName,
		Items:     append([]domain.ItemLote(nil), items...),
	}
	s.byID[id] = job
	s.fifo = append(s.fifo, id)
	return cloneJob(job), nil
}

func (s *JobStore) Get(id string) (domain.Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.byID[id]
	if !ok {
		return domain.Job{}, false
	}
	return cloneJob(job), true
}

func (s *JobStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.byID)
}

func (s *JobStore) Running() (domain.Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runningID == "" {
		return domain.Job{}, false
	}
	job, ok := s.byID[s.runningID]
	if !ok || job.Status != "running" {
		s.runningID = ""
		return domain.Job{}, false
	}
	return cloneJob(job), true
}

func (s *JobStore) ClaimNext() (domain.Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runningID != "" {
		return domain.Job{}, false
	}
	for len(s.fifo) > 0 {
		id := s.fifo[0]
		s.fifo = s.fifo[1:]
		job, ok := s.byID[id]
		if !ok || job.Status != "queued" {
			continue
		}
		job.Status = "running"
		s.runningID = id
		return cloneJob(job), true
	}
	return domain.Job{}, false
}

func (s *JobStore) MarkProgress(id string, processadas int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.byID[id]; ok {
		job.Processadas = processadas
		job.Restantes = job.Total - processadas
	}
	return nil
}

func (s *JobStore) MarkSuccess(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.byID[id]; ok {
		job.Status = "success"
		job.Processadas = job.Total
		job.Restantes = 0
		if s.runningID == id {
			s.runningID = ""
		}
	}
	return nil
}

func (s *JobStore) MarkFailed(id string, errorMessage string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.byID[id]; ok {
		job.Status = "failed"
		job.ErrorMessage = errorMessage
		job.Restantes = job.Total - job.Processadas
		if s.runningID == id {
			s.runningID = ""
		}
	}
	return nil
}

func cloneJob(job *domain.Job) domain.Job {
	cp := *job
	cp.Items = append([]domain.ItemLote(nil), job.Items...)
	cp.Restantes = cp.Total - cp.Processadas
	return cp
}
