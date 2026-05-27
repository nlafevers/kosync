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

	"kosync/internal/config"
	"kosync/internal/database"
	"kosync/internal/models"
)

func setupTestDB(t *testing.T) (*database.Storage, string) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	storage, err := database.InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	return storage, dbPath
}

func TestHandleUserCreate(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()
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
	defer storage.Close()

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
	defer storage.Close()

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
	defer storage.Close()
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
