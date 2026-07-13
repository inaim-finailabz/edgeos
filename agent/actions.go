package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"edgeos/internal/mgmtauth"
)

// ManagementEngine is what the actions handlers need to control the
// engine; a fake implementation backs handler tests without spawning a
// real llama-server.
type ManagementEngine interface {
	Stop(ctx context.Context) error
	Reload(ctx context.Context, modelPath string) error
}

// actionsMux wires the dashboard's management surface: stop the running
// engine, or reload it against a different model. Both require
// -management-token to be configured (mgmtauth 404s otherwise) and a
// matching Authorization: Bearer header.
func actionsMux(engine ManagementEngine, token string) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /v0/actions/stop", mgmtauth.RequireBearer(token, func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		if err := engine.Stop(ctx); err != nil {
			writeActionError(w, err)
			return
		}
		writeActionOK(w, "stopped")
	}))

	mux.HandleFunc("POST /v0/actions/reload", mgmtauth.RequireBearer(token, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ModelPath string `json:"model_path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ModelPath == "" {
			http.Error(w, `"model_path" is required`, http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		if err := engine.Reload(ctx, body.ModelPath); err != nil {
			writeActionError(w, err)
			return
		}
		writeActionOK(w, "reloaded")
	}))

	return mux
}

func writeActionOK(w http.ResponseWriter, status string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": status})
}

func writeActionError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
