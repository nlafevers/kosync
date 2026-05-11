package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"golang.org/x/time/rate"
)

func TestAuthMiddleware(t *testing.T) {
	dbPath := "test_middleware_auth.db"
	defer os.Remove(dbPath)

	storage, err := InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer storage.Close()

	username := "testuser"
	password := "testpass"
	hash, _ := HashPassword(password)
	if err := storage.CreateUser(username, hash); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	handler := AuthMiddleware(storage, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		user       string
		key        string
		wantStatus int
	}{
		{"Valid Auth", username, password, http.StatusOK},
		{"Invalid User", "wronguser", password, http.StatusUnauthorized},
		{"Invalid Key", username, "wrongpass", http.StatusUnauthorized},
		{"Missing Headers", "", "", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.user != "" {
				req.Header.Set("X-AUTH-USER", tt.user)
			}
			if tt.key != "" {
				req.Header.Set("X-AUTH-KEY", tt.key)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}
		})
	}
}

func TestAcceptMiddleware(t *testing.T) {
	handler := AcceptMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		accept     string
		wantStatus int
	}{
		{"Valid Accept", KOReaderMimeType, http.StatusOK},
		{"Invalid Accept", "application/json", http.StatusNotAcceptable},
		{"Missing Accept", "", http.StatusNotAcceptable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}
		})
	}
}

func TestContentTypeMiddleware(t *testing.T) {
	handler := ContentTypeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Content-Type"); got != KOReaderMimeType {
		t.Errorf("expected Content-Type %s, got %s", KOReaderMimeType, got)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	// Allow 2 requests, burst 2
	limiter := NewIPRateLimiter(rate.Limit(2), 2)
	handler := RateLimitMiddleware(limiter, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4"

	// First 2 should pass
	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i+1, rr.Code)
		}
	}

	// 3rd should be rate limited
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("request 3: expected status 429, got %d", rr.Code)
	}

	// Different IP should pass
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "5.6.7.8"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Errorf("different IP: expected status 200, got %d", rr2.Code)
	}
}
