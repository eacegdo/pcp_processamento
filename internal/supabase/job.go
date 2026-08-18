package supabase

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"

	"github.com/wellington/pcp_processamento/internal/domain"
)

// JobStore dual-writes Job metadata to pcp_job while keeping carga items in process memory.
type JobStore struct {
	client *Client

	mu        sync.Mutex
	byID      map[string]*domain.Job
	fifo      []string
	runningID string
}

func NewJobStore(supabaseURL, serviceRoleKey string, httpClient *http.Client) *JobStore {
	return &JobStore{
		client: NewClient(supabaseURL, serviceRoleKey, httpClient),
		byID:   make(map[string]*domain.Job),
	}
}

func (s *JobStore) Create(total int, fileName string, items []domain.ItemCarga) (domain.Job, error) {
	payload := map[string]any{
		"status":      "queued",
		"total":       total,
		"processadas": 0,
		"file_name":   fileName,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return domain.Job{}, err
	}
	resp, err := s.client.do(http.MethodPost, "/pcp_job", bytes.NewReader(body), "return=representation")
	if err != nil {
		return domain.Job{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.Job{}, err
	}
	var rows []struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		Total    int    `json:"total"`
		FileName string `json:"file_name"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return domain.Job{}, fmt.Errorf("decode pcp_job create: %w", err)
	}
	if len(rows) == 0 || rows[0].ID == "" {
		return domain.Job{}, fmt.Errorf("pcp_job create sem id")
	}

	job := &domain.Job{
		ID:        rows[0].ID,
		Status:    "queued",
		Total:     total,
		Restantes: total,
		FileName:  fileName,
		Items:     append([]domain.ItemCarga(nil), items...),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[job.ID] = job
	s.fifo = append(s.fifo, job.ID)
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
	if s.runningID != "" {
		s.mu.Unlock()
		return domain.Job{}, false
	}
	var job *domain.Job
	for len(s.fifo) > 0 {
		id := s.fifo[0]
		s.fifo = s.fifo[1:]
		candidate, ok := s.byID[id]
		if !ok || candidate.Status != "queued" {
			continue
		}
		job = candidate
		break
	}
	if job == nil {
		s.mu.Unlock()
		return domain.Job{}, false
	}
	id := job.ID
	s.mu.Unlock()

	if err := s.patch(id, map[string]any{"status": "running"}); err != nil {
		s.mu.Lock()
		s.fifo = append([]string{id}, s.fifo...)
		s.mu.Unlock()
		return domain.Job{}, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.byID[id]
	if !ok {
		return domain.Job{}, false
	}
	job.Status = "running"
	s.runningID = id
	return cloneJob(job), true
}

func (s *JobStore) MarkProgress(id string, processadas int) error {
	if err := s.patch(id, map[string]any{"processadas": processadas}); err != nil {
		return err
	}
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
	total := 0
	if job, ok := s.byID[id]; ok {
		total = job.Total
	}
	s.mu.Unlock()

	if err := s.patch(id, map[string]any{"status": "success", "processadas": total}); err != nil {
		return err
	}
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
	if err := s.patch(id, map[string]any{"status": "failed", "error_message": errorMessage}); err != nil {
		return err
	}
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

func (s *JobStore) patch(id string, fields map[string]any) error {
	body, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	q := url.Values{}
	q.Set("id", "eq."+id)
	resp, err := s.client.do(http.MethodPatch, "/pcp_job?"+q.Encode(), bytes.NewReader(body), "return=minimal")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func cloneJob(job *domain.Job) domain.Job {
	cp := *job
	cp.Items = append([]domain.ItemCarga(nil), job.Items...)
	cp.Restantes = cp.Total - cp.Processadas
	return cp
}
