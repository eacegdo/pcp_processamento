package memory

import (
	"sync"

	"github.com/google/uuid"
)

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

// ItemLote is one Escola's Situação OCE to apply in a Lote OCE.
type ItemLote struct {
	INEP     string
	Situacao SituacaoOCE
}

type JobStore struct {
	mu        sync.Mutex
	byID      map[string]*Job
	fifo      []string
	runningID string
}

func NewJobStore() *JobStore {
	return &JobStore{byID: make(map[string]*Job)}
}

func (s *JobStore) Create(total int, fileName string, items []ItemLote) Job {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.NewString()
	job := &Job{
		ID:        id,
		Status:    "queued",
		Total:     total,
		Restantes: total,
		FileName:  fileName,
		Items:     append([]ItemLote(nil), items...),
	}
	s.byID[id] = job
	s.fifo = append(s.fifo, id)
	return *job
}

func (s *JobStore) Get(id string) (Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.byID[id]
	if !ok {
		return Job{}, false
	}
	return cloneJob(job), true
}

func (s *JobStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.byID)
}

func (s *JobStore) Running() (Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runningID == "" {
		return Job{}, false
	}
	job, ok := s.byID[s.runningID]
	if !ok || job.Status != "running" {
		s.runningID = ""
		return Job{}, false
	}
	return cloneJob(job), true
}

func (s *JobStore) ClaimNext() (Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runningID != "" {
		return Job{}, false
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
	return Job{}, false
}

func (s *JobStore) MarkProgress(id string, processadas int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.byID[id]; ok {
		job.Processadas = processadas
		job.Restantes = job.Total - processadas
	}
}

func (s *JobStore) MarkSuccess(id string) {
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
}

func (s *JobStore) MarkFailed(id string, errorMessage string) {
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
}

func cloneJob(job *Job) Job {
	cp := *job
	cp.Items = append([]ItemLote(nil), job.Items...)
	cp.Restantes = cp.Total - cp.Processadas
	return cp
}
