package workflow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func setupTestAPI(t *testing.T) (*InternalAPI, *Service, *StepExecutor) {
	t.Helper()
	dir := t.TempDir()
	store := NewPersistStore(dir)
	if err := store.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	executor := &StepExecutor{
		AgentPromptFunc: func(ctx context.Context, prompt string) (string, error) {
			return "api response", nil
		},
	}
	engine := NewEngine(store, executor)
	svc := NewService(store, engine, ServiceConfig{WorkspaceDir: dir})
	if err := svc.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	api := NewInternalAPI(svc)
	return api, svc, executor
}

func TestInternalAPI_HandleRun(t *testing.T) {
	api, svc, _ := setupTestAPI(t)
	defer svc.Stop()

	wf := &Workflow{Name: "api-run", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}}
	svc.CreateWorkflow(wf)

	body := `{"name":"api-run","channel":"telegram","chat_id":"-100"}`
	req := httptest.NewRequest(http.MethodPost, "/internal/workflow/run", strings.NewReader(body))
	w := httptest.NewRecorder()

	api.handleRun(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Status = %d, want 200", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["instance_id"] == "" {
		t.Fatal("instance_id should not be empty")
	}
	if result["status"] != "running" {
		t.Fatalf("status = %q, want %q", result["status"], "running")
	}
}

func TestInternalAPI_HandleRun_MethodNotAllowed(t *testing.T) {
	api, svc, _ := setupTestAPI(t)
	defer svc.Stop()

	req := httptest.NewRequest(http.MethodGet, "/internal/workflow/run", nil)
	w := httptest.NewRecorder()

	api.handleRun(w, req)

	if w.Result().StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("Status = %d, want 405", w.Result().StatusCode)
	}
}

func TestInternalAPI_HandleRun_InvalidJSON(t *testing.T) {
	api, svc, _ := setupTestAPI(t)
	defer svc.Stop()

	req := httptest.NewRequest(http.MethodPost, "/internal/workflow/run", strings.NewReader("invalid"))
	w := httptest.NewRecorder()

	api.handleRun(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("Status = %d, want 400", w.Result().StatusCode)
	}
}

func TestInternalAPI_HandleRun_NotFound(t *testing.T) {
	api, svc, _ := setupTestAPI(t)
	defer svc.Stop()

	body := `{"name":"nonexistent"}`
	req := httptest.NewRequest(http.MethodPost, "/internal/workflow/run", strings.NewReader(body))
	w := httptest.NewRecorder()

	api.handleRun(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("Status = %d, want 400", w.Result().StatusCode)
	}
}

func TestInternalAPI_HandleStop(t *testing.T) {
	api, svc, _ := setupTestAPI(t)
	defer svc.Stop()

	// 创建一个运行中的实例然后停止
	wf := &Workflow{Name: "api-stop", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}}
	svc.CreateWorkflow(wf)

	// 无运行实例时 stop 也应正常返回
	body := `{"name":"api-stop"}`
	req := httptest.NewRequest(http.MethodPost, "/internal/workflow/stop", strings.NewReader(body))
	w := httptest.NewRecorder()

	api.handleStop(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Status = %d, want 200", resp.StatusCode)
	}
}

func TestInternalAPI_HandleStop_MethodNotAllowed(t *testing.T) {
	api, svc, _ := setupTestAPI(t)
	defer svc.Stop()

	req := httptest.NewRequest(http.MethodGet, "/internal/workflow/stop", nil)
	w := httptest.NewRecorder()

	api.handleStop(w, req)

	if w.Result().StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("Status = %d, want 405", w.Result().StatusCode)
	}
}

func TestInternalAPI_HandleInstances(t *testing.T) {
	api, svc, _ := setupTestAPI(t)
	defer svc.Stop()

	wf := &Workflow{Name: "api-inst", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}}
	svc.CreateWorkflow(wf)

	svc.RunWorkflow(context.Background(), "api-inst", "", "")
	time.Sleep(200 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/internal/workflow/instances?name=api-inst", nil)
	w := httptest.NewRecorder()

	api.handleInstances(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Status = %d, body = %s", resp.StatusCode, body)
	}
}

func TestInternalAPI_HandleInstances_MissingName(t *testing.T) {
	api, svc, _ := setupTestAPI(t)
	defer svc.Stop()

	req := httptest.NewRequest(http.MethodGet, "/internal/workflow/instances", nil)
	w := httptest.NewRecorder()

	api.handleInstances(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("Status = %d, want 400", w.Result().StatusCode)
	}
}

func TestInternalAPI_HandleInstance(t *testing.T) {
	api, svc, _ := setupTestAPI(t)
	defer svc.Stop()

	wf := &Workflow{Name: "api-single", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}}
	svc.CreateWorkflow(wf)

	instID, _ := svc.RunWorkflow(context.Background(), "api-single", "", "")
	time.Sleep(200 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/internal/workflow/instance?name=api-single&id="+instID, nil)
	w := httptest.NewRecorder()

	api.handleInstance(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Status = %d, want 200", resp.StatusCode)
	}
}

func TestInternalAPI_HandleInstance_MissingParams(t *testing.T) {
	api, svc, _ := setupTestAPI(t)
	defer svc.Stop()

	req := httptest.NewRequest(http.MethodGet, "/internal/workflow/instance?name=test", nil)
	w := httptest.NewRecorder()

	api.handleInstance(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("Status = %d, want 400", w.Result().StatusCode)
	}
}

func TestInternalAPI_HandleInstance_NotFound(t *testing.T) {
	api, svc, _ := setupTestAPI(t)
	defer svc.Stop()

	req := httptest.NewRequest(http.MethodGet, "/internal/workflow/instance?name=test&id=no-id", nil)
	w := httptest.NewRecorder()

	api.handleInstance(w, req)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("Status = %d, want 404", w.Result().StatusCode)
	}
}

func TestInternalAPI_HandleDeleteInstance(t *testing.T) {
	api, svc, _ := setupTestAPI(t)
	defer svc.Stop()

	wf := &Workflow{Name: "api-del", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}}
	svc.CreateWorkflow(wf)

	instID, _ := svc.RunWorkflow(context.Background(), "api-del", "", "")
	time.Sleep(200 * time.Millisecond)

	body := `{"name":"api-del","id":"` + instID + `"}`
	req := httptest.NewRequest(http.MethodPost, "/internal/workflow/delete_instance", strings.NewReader(body))
	w := httptest.NewRecorder()

	api.handleDeleteInstance(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Status = %d, want 200", resp.StatusCode)
	}
}

func TestInternalAPI_HandleDeleteInstance_MethodNotAllowed(t *testing.T) {
	api, svc, _ := setupTestAPI(t)
	defer svc.Stop()

	req := httptest.NewRequest(http.MethodGet, "/internal/workflow/delete_instance", nil)
	w := httptest.NewRecorder()

	api.handleDeleteInstance(w, req)

	if w.Result().StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("Status = %d, want 405", w.Result().StatusCode)
	}
}

func TestInternalAPI_HandleDeleteInstance_InvalidJSON(t *testing.T) {
	api, svc, _ := setupTestAPI(t)
	defer svc.Stop()

	req := httptest.NewRequest(http.MethodPost, "/internal/workflow/delete_instance", strings.NewReader("bad"))
	w := httptest.NewRecorder()

	api.handleDeleteInstance(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("Status = %d, want 400", w.Result().StatusCode)
	}
}
