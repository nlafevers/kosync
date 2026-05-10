package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
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

		if rr.Code != http.StatusConflict {
			t.Errorf("expected status 409, got %d", rr.Code)
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
