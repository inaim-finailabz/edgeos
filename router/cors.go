package main

import "net/http"

// withCORS allows any origin to call the router directly — the same
// posture llama-server itself takes, and necessary for browser-based
// clients (see examples/web/chat.html) since the router is meant to be a
// drop-in OpenAI-compatible endpoint, not something only same-origin code
// can reach. It's not a security boundary: the only authenticated routes
// (node create/delete/actions) still require the bearer token regardless
// of origin.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
