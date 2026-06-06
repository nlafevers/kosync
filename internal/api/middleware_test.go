package api

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateRequestID(t *testing.T) {
	t.Run("returns 32-char hex string", func(t *testing.T) {
		id := generateRequestID()
		if len(id) != 32 {
			t.Errorf("expected 32 hex chars, got %d: %q", len(id), id)
		}
		if _, err := hex.DecodeString(id); err != nil {
			t.Errorf("expected valid hex string, got %q: %v", id, err)
		}
	})

	t.Run("generates unique IDs", func(t *testing.T) {
		id1 := generateRequestID()
		id2 := generateRequestID()
		if id1 == id2 {
			t.Error("expected unique request IDs, got identical values")
		}
	})

	t.Run("request_id present in log context", func(t *testing.T) {
		handler := LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Context().Value(ContextKeyRequestID)
			if id == nil {
				t.Error("expected request_id in context")
				return
			}
			idStr, ok := id.(string)
			if !ok || len(idStr) != 32 {
				t.Errorf("expected 32-char hex request_id, got %q", idStr)
			}
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	})
}

func TestAuthMiddleware(t *testing.T) {
	storage, _ := setupTestDB(t)

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

// TestContentTypeMiddlewareSetsDecodableJSON guards the response Content-Type.
// KOReader's Spore client (pre-2026-02) only decodes a response body when the
// Content-Type contains the literal substring "application/json". The KOReader
// MIME type "application/vnd.koreader.v1+json" does NOT contain that substring,
// so using it as the response Content-Type silently breaks progress retrieval.
// The response type must therefore be plain "application/json".
func TestContentTypeMiddlewareSetsDecodableJSON(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	ContentTypeMiddleware(next).ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("response Content-Type = %q, want %q", ct, "application/json")
	}
	if !strings.Contains(ct, "application/json") {
		t.Errorf("response Content-Type %q is not decodable by KOReader's Spore client", ct)
	}
}
