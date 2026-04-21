package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/cron"
)

// cronJobResponse represents a cron job for API response
type cronJobResponse struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Enabled        bool              `json:"enabled"`
	Schedule       cron.CronSchedule `json:"schedule"`
	Payload        cron.CronPayload  `json:"payload"`
	State          cron.CronJobState `json:"state"`
	CreatedAtMS    int64             `json:"createdAtMs"`
	UpdatedAtMS    int64             `json:"updatedAtMs"`
	DeleteAfterRun bool              `json:"deleteAfterRun"`
}

// createJobRequest represents a request to create a new job
type createJobRequest struct {
	Name     string            `json:"name"`
	Schedule cron.CronSchedule `json:"schedule"`
	Message  string            `json:"message"`
	Command  string            `json:"command,omitempty"`
	Channel  string            `json:"channel"`
	To       string            `json:"to"`
}

// updateJobRequest represents a request to update a job
type updateJobRequest struct {
	Name     string             `json:"name"`
	Enabled  *bool              `json:"enabled,omitempty"`
	Schedule *cron.CronSchedule `json:"schedule,omitempty"`
	Message  string             `json:"message"`
	Command  string             `json:"command,omitempty"`
}

func (h *Handler) registerCronRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/cron/jobs", h.handleListCronJobs)
	mux.HandleFunc("POST /api/cron/jobs", h.handleCreateCronJob)
	mux.HandleFunc("GET /api/cron/jobs/{id}", h.handleGetCronJob)
	mux.HandleFunc("PUT /api/cron/jobs/{id}", h.handleUpdateCronJob)
	mux.HandleFunc("DELETE /api/cron/jobs/{id}", h.handleDeleteCronJob)
	mux.HandleFunc("POST /api/cron/jobs/{id}/toggle", h.handleToggleCronJob)
}

// getCronStorePath returns the path to the cron store file
func (h *Handler) getCronStorePath() (string, error) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		return "", err
	}
	workspace := cfg.WorkspacePath()
	if workspace == "" {
		workspace = ".picoclaw/workspace"
	}
	return filepath.Join(workspace, "cron", "jobs.json"), nil
}

// getCronService creates a temporary CronService for reading/writing jobs
func (h *Handler) getCronService() (*cron.CronService, error) {
	storePath, err := h.getCronStorePath()
	if err != nil {
		return nil, err
	}
	// Create service without handler (we only need CRUD operations)
	return cron.NewCronService(storePath, nil), nil
}

func (h *Handler) handleListCronJobs(w http.ResponseWriter, r *http.Request) {
	cs, err := h.getCronService()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to initialize cron service: %v", err), http.StatusInternalServerError)
		return
	}

	if err := cs.Load(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to load cron jobs: %v", err), http.StatusInternalServerError)
		return
	}

	jobs := cs.ListJobs(true)
	response := make([]cronJobResponse, len(jobs))
	for i, job := range jobs {
		response[i] = cronJobResponse(job)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"jobs": response,
	})
}

func (h *Handler) handleGetCronJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	cs, err := h.getCronService()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to initialize cron service: %v", err), http.StatusInternalServerError)
		return
	}

	if err := cs.Load(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to load cron jobs: %v", err), http.StatusInternalServerError)
		return
	}

	jobs := cs.ListJobs(true)
	for _, job := range jobs {
		if job.ID == jobID {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(cronJobResponse(job))
			return
		}
	}

	http.Error(w, "Job not found", http.StatusNotFound)
}

func (h *Handler) handleCreateCronJob(w http.ResponseWriter, r *http.Request) {
	var req createJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	cs, err := h.getCronService()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to initialize cron service: %v", err), http.StatusInternalServerError)
		return
	}

	if loadErr := cs.Load(); loadErr != nil {
		http.Error(w, fmt.Sprintf("Failed to load cron jobs: %v", loadErr), http.StatusInternalServerError)
		return
	}

	job, err := cs.AddJob(req.Name, req.Schedule, req.Message, req.Channel, req.To)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create job: %v", err), http.StatusInternalServerError)
		return
	}

	// Update payload if command is set
	if req.Command != "" {
		job.Payload.Command = req.Command
		job.Payload.Kind = "command"
		if updateErr := cs.UpdateJob(job); updateErr != nil {
			http.Error(w, fmt.Sprintf("Failed to update job payload: %v", updateErr), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(cronJobResponse(*job))
}

func (h *Handler) handleUpdateCronJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	var req updateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	cs, err := h.getCronService()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to initialize cron service: %v", err), http.StatusInternalServerError)
		return
	}

	if err := cs.Load(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to load cron jobs: %v", err), http.StatusInternalServerError)
		return
	}

	jobs := cs.ListJobs(true)
	var targetJob *cron.CronJob
	for i := range jobs {
		if jobs[i].ID == jobID {
			targetJob = &jobs[i]
			break
		}
	}

	if targetJob == nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	// Update fields
	if req.Name != "" {
		targetJob.Name = req.Name
	}
	if req.Enabled != nil {
		targetJob.Enabled = *req.Enabled
	}
	if req.Schedule != nil {
		targetJob.Schedule = *req.Schedule
	}
	if req.Message != "" {
		targetJob.Payload.Message = req.Message
	}
	if req.Command != "" {
		targetJob.Payload.Command = req.Command
		targetJob.Payload.Kind = "command"
	}

	if err := cs.UpdateJob(targetJob); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update job: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cronJobResponse(*targetJob))
}

func (h *Handler) handleDeleteCronJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	cs, err := h.getCronService()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to initialize cron service: %v", err), http.StatusInternalServerError)
		return
	}

	if err := cs.Load(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to load cron jobs: %v", err), http.StatusInternalServerError)
		return
	}

	if !cs.RemoveJob(jobID) {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) handleToggleCronJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	cs, err := h.getCronService()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to initialize cron service: %v", err), http.StatusInternalServerError)
		return
	}

	if err := cs.Load(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to load cron jobs: %v", err), http.StatusInternalServerError)
		return
	}

	job := cs.EnableJob(jobID, req.Enabled)
	if job == nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cronJobResponse(*job))
}
