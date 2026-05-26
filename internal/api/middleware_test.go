package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	// Seed user
	hash, _ := HashPassword("testpass")
	storage.CreateUser("testuser", hash)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("Valid Auth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-AUTH-USER", "testuser")
		req.Header.Set("X-AUTH-KEY", "testpass")
		w := httptest.NewRecorder()

		AuthMiddleware(storage, nextHandler).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
	})

	t.Run("Invalid Password", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-AUTH-USER", "testuser")
		req.Header.Set("X-AUTH-KEY", "wrongpass")
		w := httptest.NewRecorder()

		AuthMiddleware(storage, nextHandler).ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	t.Run("Missing Headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		AuthMiddleware(storage, nextHandler).ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})
}

func TestAcceptMiddleware(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("Valid Accept", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept", KOReaderMimeType)
		w := httptest.NewRecorder()

		AcceptMiddleware(nextHandler).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
	})

	t.Run("Invalid Accept", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		AcceptMiddleware(nextHandler).ServeHTTP(w, req)

		if w.Code != http.StatusNotAcceptable {
			t.Errorf("expected 406 Not Acceptable, got %d", w.Code)
		}
	})
}
