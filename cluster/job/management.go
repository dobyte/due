package job

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xuuid"
)

// Manager is the storage-neutral management surface exposed by Service.
type Manager interface {
	Create(ctx context.Context, definition *Definition) error
	GetDefinition(ctx context.Context, jobID string) (*Definition, error)
	ListDueDefinitions(ctx context.Context, req ListDueRequest) ([]*Definition, error)
	UpdateDefinition(ctx context.Context, definition *Definition, version int64) error
	PauseDefinition(ctx context.Context, jobID string, version int64) error
	ResumeDefinition(ctx context.Context, jobID string, version int64) error
	DeleteDefinition(ctx context.Context, jobID string, version int64) error
	Trigger(ctx context.Context, jobID string) (*Run, error)
	GetRun(ctx context.Context, runID string) (*Run, error)
	CancelRun(ctx context.Context, runID string) error
	RetryRun(ctx context.Context, runID string) (*Run, error)
	GetStats(ctx context.Context, req StatsRequest) (*Stats, error)
}

var _ Manager = (*Service)(nil)

// GetDefinition returns one job definition.
func (s *Service) GetDefinition(ctx context.Context, jobID string) (*Definition, error) {
	if s == nil || s.store == nil {
		return nil, errors.ErrInvalidArgument
	}
	return s.store.GetDefinition(ctx, jobID)
}

// ListDueDefinitions lists definitions whose next run is due.
func (s *Service) ListDueDefinitions(ctx context.Context, req ListDueRequest) ([]*Definition, error) {
	if s == nil || s.store == nil {
		return nil, errors.ErrInvalidArgument
	}
	return s.store.ListDueDefinitions(ctx, req)
}

// GetRun returns one run by ID.
func (s *Service) GetRun(ctx context.Context, runID string) (*Run, error) {
	if s == nil || s.store == nil {
		return nil, errors.ErrInvalidArgument
	}
	return s.store.GetRun(ctx, runID)
}

// GetStats returns coarse queue metrics from the backing store.
func (s *Service) GetStats(ctx context.Context, req StatsRequest) (*Stats, error) {
	if s == nil || s.store == nil {
		return nil, errors.ErrInvalidArgument
	}
	return s.store.GetStats(ctx, req)
}

// UpdateDefinition updates a definition using optimistic version checking.
func (s *Service) UpdateDefinition(ctx context.Context, definition *Definition, version int64) error {
	if s == nil || s.store == nil {
		return errors.ErrInvalidArgument
	}
	return s.store.UpdateDefinition(ctx, definition, version)
}

// PauseDefinition pauses a definition using optimistic version checking.
func (s *Service) PauseDefinition(ctx context.Context, jobID string, version int64) error {
	if s == nil || s.store == nil {
		return errors.ErrInvalidArgument
	}
	return s.store.PauseDefinition(ctx, jobID, version)
}

// ResumeDefinition resumes a definition using optimistic version checking.
func (s *Service) ResumeDefinition(ctx context.Context, jobID string, version int64) error {
	if s == nil || s.store == nil {
		return errors.ErrInvalidArgument
	}
	return s.store.ResumeDefinition(ctx, jobID, version)
}

// DeleteDefinition marks a definition deleted using optimistic version checking.
func (s *Service) DeleteDefinition(ctx context.Context, jobID string, version int64) error {
	if s == nil || s.store == nil {
		return errors.ErrInvalidArgument
	}
	return s.store.DeleteDefinition(ctx, jobID, version)
}

// CancelRun cancels a run by ID.
func (s *Service) CancelRun(ctx context.Context, runID string) error {
	if s == nil || s.store == nil {
		return errors.ErrInvalidArgument
	}
	return s.store.CancelRun(ctx, CancelRunRequest{RunID: runID})
}

// RetryRun creates a new pending run from a dead or canceled run.
func (s *Service) RetryRun(ctx context.Context, runID string) (*Run, error) {
	if s == nil || s.store == nil {
		return nil, errors.ErrInvalidArgument
	}
	previous, err := s.store.GetRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	if previous == nil {
		return nil, ErrRunNotFound
	}
	if previous.Status != RunDead && previous.Status != RunCanceled {
		return nil, ErrInvalidRun
	}

	now := time.Now()
	run := &Run{
		ID:                xuuid.UUID(),
		JobID:             previous.JobID,
		TaskType:          previous.TaskType,
		Payload:           append([]byte(nil), previous.Payload...),
		ScheduledAt:       now.Add(time.Second),
		ReadyAt:           now,
		Timeout:           previous.Timeout,
		Status:            RunPending,
		ConcurrencyPolicy: previous.ConcurrencyPolicy,
		MaxAttempts:       previous.MaxAttempts,
		RetryPolicy:       previous.RetryPolicy,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.store.CreateRun(ctx, run); err != nil {
		return nil, err
	}
	if s.opts != nil && s.opts.notifier != nil {
		if err := s.opts.notifier.Notify(ctx, run.JobID); err != nil {
			return nil, err
		}
	}
	return run, nil
}

// ManagementHandler exposes the storage-neutral management API over net/http.
type ManagementHandler struct {
	manager Manager
}

// NewManagementHandler creates a management HTTP handler.
func NewManagementHandler(manager Manager) *ManagementHandler {
	return &ManagementHandler{manager: manager}
}

// ServeHTTP routes management requests. Authentication and authorization must
// be applied by the embedding application or an outer middleware.
func (h *ManagementHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if h == nil || h.manager == nil {
		writeManagementError(writer, http.StatusInternalServerError, errors.ErrInvalidArgument)
		return
	}
	path := strings.Trim(request.URL.Path, "/")
	parts := strings.Split(path, "/")
	if path == "stats" && request.Method == http.MethodGet {
		stats, err := h.manager.GetStats(request.Context(), StatsRequest{})
		if err != nil {
			writeManagementError(writer, managementStatus(err), err)
			return
		}
		writeManagementJSON(writer, http.StatusOK, stats)
		return
	}
	if len(parts) == 1 && parts[0] == "jobs" {
		h.handleJobs(writer, request)
		return
	}
	if len(parts) == 2 && parts[0] == "jobs" {
		h.handleJob(writer, request, parts[1])
		return
	}
	if len(parts) == 3 && parts[0] == "jobs" && parts[2] == "trigger" && request.Method == http.MethodPost {
		run, err := h.manager.Trigger(request.Context(), parts[1])
		if err != nil {
			writeManagementError(writer, managementStatus(err), err)
			return
		}
		writeManagementJSON(writer, http.StatusCreated, run)
		return
	}
	if len(parts) == 3 && parts[0] == "jobs" && request.Method == http.MethodPost {
		h.handleJobAction(writer, request, parts[1], parts[2])
		return
	}
	if len(parts) == 2 && parts[0] == "runs" {
		h.handleRun(writer, request, parts[1])
		return
	}
	if len(parts) == 3 && parts[0] == "runs" {
		h.handleRunAction(writer, request, parts[1], parts[2])
		return
	}
	http.NotFound(writer, request)
}

func (h *ManagementHandler) handleJobs(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		definitions, err := h.manager.ListDueDefinitions(request.Context(), ListDueRequest{})
		if err != nil {
			writeManagementError(writer, managementStatus(err), err)
			return
		}
		writeManagementJSON(writer, http.StatusOK, definitions)
	case http.MethodPost:
		var definition Definition
		if err := readManagementJSON(request, &definition); err != nil {
			writeManagementError(writer, http.StatusBadRequest, err)
			return
		}
		if err := h.manager.Create(request.Context(), &definition); err != nil {
			writeManagementError(writer, managementStatus(err), err)
			return
		}
		writeManagementJSON(writer, http.StatusCreated, &definition)
	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *ManagementHandler) handleJob(writer http.ResponseWriter, request *http.Request, jobID string) {
	switch request.Method {
	case http.MethodGet:
		definition, err := h.manager.GetDefinition(request.Context(), jobID)
		if err != nil {
			writeManagementError(writer, managementStatus(err), err)
			return
		}
		writeManagementJSON(writer, http.StatusOK, definition)
	case http.MethodPut:
		version, err := managementVersion(request)
		if err != nil {
			writeManagementError(writer, http.StatusBadRequest, err)
			return
		}
		var definition Definition
		if err := readManagementJSON(request, &definition); err != nil {
			writeManagementError(writer, http.StatusBadRequest, err)
			return
		}
		definition.ID = jobID
		if err := h.manager.UpdateDefinition(request.Context(), &definition, version); err != nil {
			writeManagementError(writer, managementStatus(err), err)
			return
		}
		writeManagementJSON(writer, http.StatusOK, &definition)
	case http.MethodDelete:
		version, err := managementVersion(request)
		if err != nil {
			writeManagementError(writer, http.StatusBadRequest, err)
			return
		}
		if err := h.manager.DeleteDefinition(request.Context(), jobID, version); err != nil {
			writeManagementError(writer, managementStatus(err), err)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *ManagementHandler) handleJobAction(writer http.ResponseWriter, request *http.Request, jobID, action string) {
	version, err := managementVersion(request)
	if err != nil {
		writeManagementError(writer, http.StatusBadRequest, err)
		return
	}
	switch action {
	case "pause":
		err = h.manager.PauseDefinition(request.Context(), jobID, version)
	case "resume":
		err = h.manager.ResumeDefinition(request.Context(), jobID, version)
	default:
		http.NotFound(writer, request)
		return
	}
	if err != nil {
		writeManagementError(writer, managementStatus(err), err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (h *ManagementHandler) handleRun(writer http.ResponseWriter, request *http.Request, runID string) {
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	run, err := h.manager.GetRun(request.Context(), runID)
	if err != nil {
		writeManagementError(writer, managementStatus(err), err)
		return
	}
	writeManagementJSON(writer, http.StatusOK, run)
}

func (h *ManagementHandler) handleRunAction(writer http.ResponseWriter, request *http.Request, runID, action string) {
	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	switch action {
	case "cancel":
		if err := h.manager.CancelRun(request.Context(), runID); err != nil {
			writeManagementError(writer, managementStatus(err), err)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	case "retry":
		run, err := h.manager.RetryRun(request.Context(), runID)
		if err != nil {
			writeManagementError(writer, managementStatus(err), err)
			return
		}
		writeManagementJSON(writer, http.StatusCreated, run)
	default:
		http.NotFound(writer, request)
	}
}

func readManagementJSON(request *http.Request, value any) error {
	defer request.Body.Close()
	data, err := io.ReadAll(request.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}

func writeManagementJSON(writer http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		writeManagementError(writer, http.StatusInternalServerError, err)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write(data)
}

func writeManagementError(writer http.ResponseWriter, status int, err error) {
	writeManagementJSON(writer, status, map[string]string{"error": err.Error()})
}

func managementVersion(request *http.Request) (int64, error) {
	value := request.URL.Query().Get("version")
	if value == "" {
		return 0, errors.ErrInvalidArgument
	}
	return strconv.ParseInt(value, 10, 64)
}

func managementStatus(err error) int {
	switch {
	case errors.Is(err, ErrDefinitionNotFound), errors.Is(err, ErrRunNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrVersionConflict):
		return http.StatusConflict
	case errors.Is(err, errors.ErrInvalidArgument), errors.Is(err, ErrInvalidDefinition), errors.Is(err, ErrInvalidRun):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
