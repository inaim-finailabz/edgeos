package mgmtauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func called(t *testing.T) (http.HandlerFunc, *bool) {
	hit := false
	return func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	}, &hit
}

func TestRequireBearer_NoTokenConfigured_Disabled(t *testing.T) {
	next, hit := called(t)
	handler := RequireBearer("", next)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer anything")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if *hit {
		t.Error("handler should not run when no token is configured")
	}
}

func TestRequireBearer_MissingHeader_Unauthorized(t *testing.T) {
	next, hit := called(t)
	handler := RequireBearer("secret", next)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if *hit {
		t.Error("handler should not run without a token")
	}
}

func TestRequireBearer_WrongToken_Unauthorized(t *testing.T) {
	next, hit := called(t)
	handler := RequireBearer("secret", next)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if *hit {
		t.Error("handler should not run with a wrong token")
	}
}

func TestRequireBearer_CorrectToken_Runs(t *testing.T) {
	next, hit := called(t)
	handler := RequireBearer("secret", next)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !*hit {
		t.Error("handler should run with the correct token")
	}
}
