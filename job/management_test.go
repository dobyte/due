package job

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type managementManager struct {
	definition  *Definition
	run         *Run
	stats       *Stats
	created     bool
	triggered   bool
	canceled    bool
	deletedWith int64
}

func (m *managementManager) Create(_ context.Context, definition *Definition) error {
	m.created = true
	if definition.Version == 0 {
		definition.Version = 1
	}
	m.definition = definition
	return nil
}

func (m *managementManager) GetDefinition(context.Context, string) (*Definition, error) {
	return m.definition, nil
}

func (m *managementManager) ListDueDefinitions(context.Context, ListDueRequest) ([]*Definition, error) {
	if m.definition == nil {
		return nil, nil
	}
	return []*Definition{m.definition}, nil
}

func (m *managementManager) UpdateDefinition(_ context.Context, definition *Definition, _ int64) error {
	m.definition = definition
	return nil
}

func (m *managementManager) PauseDefinition(context.Context, string, int64) error  { return nil }
func (m *managementManager) ResumeDefinition(context.Context, string, int64) error { return nil }

func (m *managementManager) DeleteDefinition(_ context.Context, _ string, version int64) error {
	m.deletedWith = version
	return nil
}

func (m *managementManager) Trigger(context.Context, string) (*Run, error) {
	m.triggered = true
	return m.run, nil
}

func (m *managementManager) GetRun(context.Context, string) (*Run, error) { return m.run, nil }

func (m *managementManager) CancelRun(context.Context, string) error {
	m.canceled = true
	return nil
}

func (m *managementManager) RetryRun(context.Context, string) (*Run, error) { return m.run, nil }
func (m *managementManager) GetStats(context.Context, StatsRequest) (*Stats, error) {
	return m.stats, nil
}

func TestManagementHandlerRoutes(t *testing.T) {
	manager := &managementManager{
		run:   &Run{ID: "run-1", JobID: "job-1", Status: RunPending, CreatedAt: time.Now()},
		stats: &Stats{Pending: 2, Running: 1},
	}
	handler := NewManagementHandler(manager)

	create := httptest.NewRequest(http.MethodPost, "/jobs", strings.NewReader(`{"ID":"job-1","Type":"email","ScheduleType":"once"}`))
	createRecorder := httptest.NewRecorder()
	handler.ServeHTTP(createRecorder, create)
	if createRecorder.Code != http.StatusCreated || !manager.created {
		t.Fatalf("create status=%d created=%v body=%s", createRecorder.Code, manager.created, createRecorder.Body.String())
	}

	get := httptest.NewRequest(http.MethodGet, "/jobs/job-1", nil)
	getRecorder := httptest.NewRecorder()
	handler.ServeHTTP(getRecorder, get)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", getRecorder.Code, getRecorder.Body.String())
	}

	trigger := httptest.NewRequest(http.MethodPost, "/jobs/job-1/trigger", nil)
	triggerRecorder := httptest.NewRecorder()
	handler.ServeHTTP(triggerRecorder, trigger)
	if triggerRecorder.Code != http.StatusCreated || !manager.triggered {
		t.Fatalf("trigger status=%d triggered=%v", triggerRecorder.Code, manager.triggered)
	}

	cancel := httptest.NewRequest(http.MethodPost, "/runs/run-1/cancel", nil)
	cancelRecorder := httptest.NewRecorder()
	handler.ServeHTTP(cancelRecorder, cancel)
	if cancelRecorder.Code != http.StatusNoContent || !manager.canceled {
		t.Fatalf("cancel status=%d canceled=%v", cancelRecorder.Code, manager.canceled)
	}

	stats := httptest.NewRequest(http.MethodGet, "/stats", nil)
	statsRecorder := httptest.NewRecorder()
	handler.ServeHTTP(statsRecorder, stats)
	if statsRecorder.Code != http.StatusOK {
		t.Fatalf("stats status=%d body=%s", statsRecorder.Code, statsRecorder.Body.String())
	}
}

func TestManagementHandlerDeleteRequiresVersion(t *testing.T) {
	manager := &managementManager{}
	handler := NewManagementHandler(manager)
	request := httptest.NewRequest(http.MethodDelete, "/jobs/job-1", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodDelete, "/jobs/job-1?version=7", nil)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent || manager.deletedWith != 7 {
		t.Fatalf("status=%d deletedWith=%d", recorder.Code, manager.deletedWith)
	}
}
