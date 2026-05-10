package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestHandleUserCreate(t *testing.T) {
	dbPath := "test_handlers_user.db"
	defer os.Remove(dbPath)

	storage, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer storage.Close()

	config := &Config{DisableRegistration: false}
	handler := handleUserCreate(storage, config)

	t.Run("Successful Registration", func(t *testing.T) {
		reqBody, _ := json.Marshal(UserCreateRequest{
			Username: "newuser",
			Password: "md5password",
		})
		req := httptest.NewRequest("POST", "/users/create", bytes.NewBuffer(reqBody))
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d", rr.Code)
		}

		var resp UserCreateResponse
		json.NewDecoder(rr.Body).Decode(&resp)
		if resp.Username != "newuser" {
			t.Errorf("expected username newuser, got %s", resp.Username)
		}

		// Verify in DB
		hash, err := storage.GetUserHash("newuser")
		if err != nil {
			t.Errorf("failed to get hash from db: %v", err)
		}
		if !CheckPassword(hash, "md5password") {
			t.Error("stored password check failed")
		}
	})

	t.Run("Duplicate Registration", func(t *testing.T) {
		reqBody, _ := json.Marshal(UserCreateRequest{
			Username: "newuser",
			Password: "md5password",
		})
		req := httptest.NewRequest("POST", "/users/create", bytes.NewBuffer(reqBody))
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d", rr.Code)
		}
	})

	t.Run("Disabled Registration", func(t *testing.T) {
		config.DisableRegistration = true
		reqBody, _ := json.Marshal(UserCreateRequest{
			Username: "anotheruser",
			Password: "md5password",
		})
		req := httptest.NewRequest("POST", "/users/create", bytes.NewBuffer(reqBody))
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", rr.Code)
		}
	})
}

func TestHandleAuth(t *testing.T) {
	req := httptest.NewRequest("GET", "/users/auth", nil)
	rr := httptest.NewRecorder()

	handleAuth(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["authorized"] != "OK" {
		t.Errorf("expected authorized: OK, got %s", resp["authorized"])
	}
}

func TestHandleGetProgress(t *testing.T) {
	dbPath := "test_handlers_progress.db"
	defer os.Remove(dbPath)

	storage, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer storage.Close()

	username := "testuser"
	docID := "testdoc"
	progress := Progress{
		Document:   docID,
		Percentage: 0.5,
		Timestamp:  time.Now().Unix(),
	}

	// Setup: create user and some progress
	storage.CreateUser(username, "hash")
	storage.UpsertProgress(username, progress)

	handler := handleGetProgress(storage)

	t.Run("Progress Found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/syncs/progress/"+docID, nil)
		req.Header.Set("X-AUTH-USER", username)
		req.SetPathValue("document", docID) // Go 1.22 testing
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		var resp Progress
		json.NewDecoder(rr.Body).Decode(&resp)
		if resp.Document != docID || resp.Percentage != 0.5 {
			t.Errorf("unexpected progress data: %+v", resp)
		}
	})

	t.Run("Progress Not Found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/syncs/progress/unknown", nil)
		req.Header.Set("X-AUTH-USER", username)
		req.SetPathValue("document", "unknown")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", rr.Code)
		}
	})
}

func TestHandleUpdateProgress(t *testing.T) {
	dbPath := "test_handlers_update.db"
	defer os.Remove(dbPath)

	storage, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer storage.Close()

	username := "testuser"
	storage.CreateUser(username, "hash")

	handler := handleUpdateProgress(storage)

	t.Run("Successful Update", func(t *testing.T) {
		p := Progress{
			Document:   "testdoc",
			Percentage: 0.8,
			Progress:   "/page/10",
			DeviceID:   "dev123",
			Device:     "kindle",
		}
		reqBody, _ := json.Marshal(p)
		req := httptest.NewRequest("PUT", "/syncs/progress", bytes.NewBuffer(reqBody))
		req.Header.Set("X-AUTH-USER", username)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		// Verify in DB
		got, _ := storage.GetProgress(username, "testdoc")
		if got.Percentage != 0.8 || got.DeviceID != "dev123" {
			t.Errorf("unexpected progress data in db: %+v", got)
		}
	})

	t.Run("Missing Document ID", func(t *testing.T) {
		reqBody, _ := json.Marshal(Progress{Percentage: 0.5})
		req := httptest.NewRequest("PUT", "/syncs/progress", bytes.NewBuffer(reqBody))
		req.Header.Set("X-AUTH-USER", username)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rr.Code)
		}
	})

	t.Run("Invalid Percentage", func(t *testing.T) {
		reqBody, _ := json.Marshal(Progress{Document: "doc1", Percentage: 1.5})
		req := httptest.NewRequest("PUT", "/syncs/progress", bytes.NewBuffer(reqBody))
		req.Header.Set("X-AUTH-USER", username)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rr.Code)
		}
	})
}
