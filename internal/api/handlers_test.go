package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"kosync/internal/config"
	"kosync/internal/database"
	"kosync/internal/models"

	"golang.org/x/time/rate"
)

func setupTestDB(t *testing.T) (*database.Storage, string) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := database.OpenSQLite(dbPath, true)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		db.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}
	storage := database.NewStorage(db, slog.Default())
	t.Cleanup(func() { db.Close() })
	return storage, dbPath
}

func TestHandleUserCreate(t *testing.T) {
	storage, _ := setupTestDB(t)
	cfg := &config.Config{DisableRegistration: false}

	t.Run("Successful Registration", func(t *testing.T) {
		reqBody, _ := json.Marshal(UserCreateRequest{Username: "testuser", Password: "testpassword"})
		req := httptest.NewRequest("POST", "/users/create", bytes.NewBuffer(reqBody))
		w := httptest.NewRecorder()

		handler := HandleUserCreate(storage, cfg)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("expected 201 Created, got %d", w.Code)
		}

		var resp UserCreateResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if resp.Username != "testuser" {
			t.Errorf("expected testuser, got %s", resp.Username)
		}
	})

	t.Run("Registration Disabled", func(t *testing.T) {
		disabledCfg := &config.Config{DisableRegistration: true}
		reqBody, _ := json.Marshal(UserCreateRequest{Username: "otheruser", Password: "testpassword"})
		req := httptest.NewRequest("POST", "/users/create", bytes.NewBuffer(reqBody))
		w := httptest.NewRecorder()

		handler := HandleUserCreate(storage, disabledCfg)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", w.Code)
		}
	})

	t.Run("Existing User", func(t *testing.T) {
		// First registration already happened in the first test case
		reqBody, _ := json.Marshal(UserCreateRequest{Username: "testuser", Password: "newpassword"})
		req := httptest.NewRequest("POST", "/users/create", bytes.NewBuffer(reqBody))
		w := httptest.NewRecorder()

		handler := HandleUserCreate(storage, cfg)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("expected 201 Created even for existing user, got %d", w.Code)
		}
	})
}

func TestHandleAuth(t *testing.T) {
	req := httptest.NewRequest("GET", "/users/auth", nil)
	w := httptest.NewRecorder()

	HandleAuth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["authorized"] != "OK" {
		t.Errorf("expected authorized: OK, got %s", resp["authorized"])
	}
}

func TestHandleGetProgress(t *testing.T) {
	storage, _ := setupTestDB(t)
	// Seed data
	hash, _ := HashPassword("testpass")
	storage.CreateUser("testuser", hash)
	storage.UpsertProgress("testuser", models.Progress{Document: "doc1", Percentage: 0.5})

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/syncs/progress/doc1", nil)
		req.Header.Set("X-AUTH-USER", "testuser")
		req.SetPathValue("document", "doc1")
		w := httptest.NewRecorder()

		handler := HandleGetProgress(storage)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}

		var p models.Progress
		json.NewDecoder(w.Body).Decode(&p)
		if p.Document != "doc1" || p.Percentage != 0.5 {
			t.Errorf("unexpected progress data: %+v", p)
		}
	})

	t.Run("Not Found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/syncs/progress/unknown", nil)
		req.Header.Set("X-AUTH-USER", "testuser")
		req.SetPathValue("document", "unknown")
		w := httptest.NewRecorder()

		handler := HandleGetProgress(storage)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d", w.Code)
		}
	})
}

func TestHandleAuthLogsWithRequestContext(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	oldDefault := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldDefault)

	handler := LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), ContextKeyUser, "testuser")
		HandleAuth(w, r.WithContext(ctx))
	}))

	req := httptest.NewRequest("GET", "/users/auth", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	logs := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	foundAuth := false
	for _, line := range logs {
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("failed to parse log line %s: %v", string(line), err)
		}
		if entry["msg"] == "auth successful" {
			foundAuth = true
			if entry["username"] != "testuser" {
				t.Fatalf("expected username testuser, got %v", entry["username"])
			}
			if entry["request_id"] == nil {
				t.Fatal("expected request_id in auth log")
			}
		}
	}

	if !foundAuth {
		t.Fatal("expected auth successful log entry")
	}
}

func TestHandleGetProgressLogsSuccess(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	oldDefault := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldDefault)

	storage, _ := setupTestDB(t)
	hash, _ := HashPassword("testpass")
	storage.CreateUser("testuser", hash)
	storage.UpsertProgress("testuser", models.Progress{Document: "doc1", Percentage: 0.5})

	handler := LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), ContextKeyUser, "testuser")
		HandleGetProgress(storage)(w, r.WithContext(ctx))
	}))

	req := httptest.NewRequest("GET", "/syncs/progress/doc1", nil)
	req.SetPathValue("document", "doc1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	logs := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	foundProgress := false
	for _, line := range logs {
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("failed to parse log line %s: %v", string(line), err)
		}
		if entry["msg"] == "progress retrieved" {
			foundProgress = true
			if entry["username"] != "testuser" {
				t.Fatalf("expected username testuser, got %v", entry["username"])
			}
			if entry["document"] != "doc1" {
				t.Fatalf("expected document doc1, got %v", entry["document"])
			}
			if entry["request_id"] == nil {
				t.Fatal("expected request_id in progress log")
			}
		}
	}

	if !foundProgress {
		t.Fatal("expected progress retrieved log entry")
	}
}

func TestHandleUpdateProgress(t *testing.T) {
	storage, dbPath := setupTestDB(t)
	cfg := &config.Config{DatabasePath: dbPath, StorageCapMB: 0}

	t.Run("Success", func(t *testing.T) {
		p := models.Progress{Document: "doc2", Percentage: 0.8}
		reqBody, _ := json.Marshal(p)
		req := httptest.NewRequest("PUT", "/syncs/progress", bytes.NewBuffer(reqBody))
		req.Header.Set("X-AUTH-USER", "testuser")
		w := httptest.NewRecorder()

		handler := HandleUpdateProgress(storage, cfg)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}

		// Verify in DB
		saved, _ := storage.GetProgress("testuser", "doc2")
		if saved == nil || saved.Percentage != 0.8 {
			t.Error("progress not saved correctly")
		}
	})

	t.Run("Invalid Percentage", func(t *testing.T) {
		p := models.Progress{Document: "doc2", Percentage: 1.5}
		reqBody, _ := json.Marshal(p)
		req := httptest.NewRequest("PUT", "/syncs/progress", bytes.NewBuffer(reqBody))
		w := httptest.NewRecorder()

		handler := HandleUpdateProgress(storage, cfg)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}

// TestRateLimitMiddleware verifies that requests exceeding the burst receive 429,
// and that requests within the burst succeed.
func TestRateLimitMiddleware(t *testing.T) {
	const burst = 3
	// Very slow refill (1 token per hour) so no tokens are restored during the test.
	limiter := NewIPRateLimiter(rate.Every(time.Hour), burst, false)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// Wrap in LoggingMiddleware so GetLogger does not fall back to a nil logger.
	handler := LoggingMiddleware(RateLimitMiddleware(limiter, inner))

	for i := 0; i < burst; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// The next request (beyond burst) must be rejected.
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 after exceeding burst, got %d", w.Code)
	}
}

// TestRateLimitMiddlewareXForwardedFor verifies that X-Forwarded-For is used
// when trustProxy is enabled, so clients behind a proxy are limited by real IP.
func TestRateLimitMiddlewareXForwardedFor(t *testing.T) {
	const burst = 2
	limiter := NewIPRateLimiter(rate.Every(time.Hour), burst, true)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := LoggingMiddleware(RateLimitMiddleware(limiter, inner))

	realIP := "10.0.0.1"
	for i := 0; i < burst; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "127.0.0.1:9999"
		req.Header.Set("X-Forwarded-For", realIP+", 172.16.0.1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// Another request from the same real IP is rejected.
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:9999"
	req.Header.Set("X-Forwarded-For", realIP)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for same real IP after burst, got %d", w.Code)
	}

	// A different real IP still gets its own fresh bucket.
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "127.0.0.1:9999"
	req2.Header.Set("X-Forwarded-For", "10.0.0.2")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("expected 200 for different real IP, got %d", w2.Code)
	}
}

// TestRateLimitWithinBurst verifies that requests within the burst all succeed.
func TestRateLimitWithinBurst(t *testing.T) {
	const burst = 5
	limiter := NewIPRateLimiter(rate.Every(time.Hour), burst, false)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := LoggingMiddleware(RateLimitMiddleware(limiter, inner))

	for i := 0; i < burst; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.0.2.2:5678"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d within burst: expected 200, got %d", i+1, w.Code)
		}
	}
}

func TestProgressTimestampConflict(t *testing.T) {
	storage, _ := setupTestDB(t)
	cfg := &config.Config{DatabasePath: "test.db", StorageCapMB: 10}

	username := "testuser"
	doc := "doc123"

	// Helper to send PUT request
	sendPut := func(percentage float64, timestamp int64) *httptest.ResponseRecorder {
		p := models.Progress{
			Document:   doc,
			Percentage: percentage,
			Progress:   "loc1",
			Timestamp:  timestamp,
		}
		body, _ := json.Marshal(p)
		req := httptest.NewRequest("PUT", "/syncs/progress", bytes.NewBuffer(body))
		req = req.WithContext(context.WithValue(req.Context(), ContextKeyUser, username))
		w := httptest.NewRecorder()
		HandleUpdateProgress(storage, cfg).ServeHTTP(w, req)
		return w
	}

	// Helper to get progress
	getProgress := func() *models.Progress {
		req := httptest.NewRequest("GET", "/syncs/progress/"+doc, nil)
		req = req.WithContext(context.WithValue(req.Context(), ContextKeyUser, username))
		req.SetPathValue("document", doc)
		w := httptest.NewRecorder()
		HandleGetProgress(storage).ServeHTTP(w, req)
		var p models.Progress
		json.NewDecoder(w.Body).Decode(&p)
		return &p
	}

	// 1. Initial Sync
	sendPut(0.5, 1000)

	// 2. Older Update
	sendPut(0.4, 900)
	p := getProgress()
	if p.Percentage != 0.5 {
		t.Errorf("expected 0.5, got %f (older timestamp update applied)", p.Percentage)
	}

	// 3. Newer Update
	sendPut(0.6, 1100)
	p = getProgress()
	if p.Percentage != 0.6 {
		t.Errorf("expected 0.6, got %f", p.Percentage)
	}

	// 4. Equal Timestamp (Strictly greater logic means this should NOT update)
	sendPut(0.7, 1100)
	p = getProgress()
	if p.Percentage != 0.6 {
		t.Errorf("expected 0.6, got %f (equal timestamp update applied)", p.Percentage)
	}
}
