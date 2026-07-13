// Package mgmtauth is the shared static-bearer-token check for the
// management actions the dashboard drives (stop/reload an engine, evict a
// node). It is deliberately minimal: one shared secret, no accounts or
// roles — read paths (capabilities, /v0/nodes) stay unauthenticated as
// before. Configure the same token on agent, router, and dashboard.
package mgmtauth

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// RequireBearer wraps next so it only runs when the request's
// "Authorization: Bearer <token>" matches. An empty configured token
// disables the route entirely (404) rather than accepting any request —
// management actions must be deliberately turned on.
func RequireBearer(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			http.Error(w, "management actions are disabled (no management token configured)", http.StatusNotFound)
			return
		}
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
