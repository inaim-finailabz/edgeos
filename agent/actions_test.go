package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeManagementEngine struct {
	stopErr      error
	reloadErr    error
	reloadedPath string
	stopCalled   bool
}

func (f *fakeManagementEngine) Stop(ctx context.Context) error {
	f.stopCalled = true
	return f.stopErr
}

func (f *fakeManagementEngine) Reload(ctx context.Context, modelPath string) error {
	f.reloadedPath = modelPath
	return f.reloadErr
}

func TestActionsMux_Stop_RequiresAuth(t *testing.T) {
	mux := actionsMux(&fakeManagementEngine{}, "secret")
	req := httptest.NewRequest(http.MethodPost, "/v0/actions/stop", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestActionsMux_Stop_Success(t *testing.T) {
	engine := &fakeManagementEngine{}
	mux := actionsMux(engine, "secret")

	req := httptest.NewRequest(http.MethodPost, "/v0/actions/stop", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !engine.stopCalled {
		t.Error("Stop() was not called")
	}
	var body map[string]string
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "stopped" {
		t.Errorf("status field = %q, want stopped", body["status"])
	}
}

func TestActionsMux_Stop_EngineError(t *testing.T) {
	engine := &fakeManagementEngine{stopErr: errors.New("boom")}
	mux := actionsMux(engine, "secret")

	req := httptest.NewRequest(http.MethodPost, "/v0/actions/stop", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestActionsMux_Reload_Success(t *testing.T) {
	engine := &fakeManagementEngine{}
	mux := actionsMux(engine, "secret")

	req := httptest.NewRequest(http.MethodPost, "/v0/actions/reload", strings.NewReader(`{"model_path":"/models/other.gguf"}`))
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if engine.reloadedPath != "/models/other.gguf" {
		t.Errorf("reloadedPath = %q, want /models/other.gguf", engine.reloadedPath)
	}
}

func TestActionsMux_Reload_RequiresModelPath(t *testing.T) {
	mux := actionsMux(&fakeManagementEngine{}, "secret")

	req := httptest.NewRequest(http.MethodPost, "/v0/actions/reload", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestActionsMux_DisabledWithoutToken(t *testing.T) {
	mux := actionsMux(&fakeManagementEngine{}, "")
	req := httptest.NewRequest(http.MethodPost, "/v0/actions/stop", nil)
	req.Header.Set("Authorization", "Bearer anything")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
