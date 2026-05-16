package api

import (
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"math/big"
	"net/http"
	"time"

	"kosync/internal/config"
	"kosync/internal/database"
	"kosync/internal/models"
)

type UserCreateRequest struct {
	Username string `json:"username"`
	Password string `json:"password"` // This is the MD5 from the client
}

type UserCreateResponse struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

func HandleUserCreate(storage *database.Storage, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.DisableRegistration {
			slog.Warn("registration attempt while disabled", "remote_addr", r.RemoteAddr, "source", "API")
			http.Error(w, "Registration is disabled", http.StatusForbidden)
			return
		}

		var req UserCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("failed to decode registration request", "error", err, "source", "API")
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Username == "" || req.Password == "" {
			http.Error(w, "Username and password are required", http.StatusBadRequest)
			return
		}

		// Check if user already exists
		existingHash, err := storage.GetUserHash(req.Username)
		if err == nil && existingHash != "" {
			slog.Info("registration attempt for existing user", "username", req.Username, "source", "API")
			// Random delay to prevent timing attacks, then pretend it succeeded
			randomDelay()
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(UserCreateResponse{
				Username: req.Username,
				Message:  "User created",
			})
			return
		}

		// Hash the password (which is already an MD5 from the client) using Bcrypt
		hash, err := HashPassword(req.Password)
		if err != nil {
			slog.Error("failed to hash password", "username", req.Username, "error", err, "source", "API")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if err := storage.CreateUser(req.Username, hash); err != nil {
			slog.Error("failed to create user", "username", req.Username, "error", err, "source", "API")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		slog.Info("user created successfully", "username", req.Username, "source", "API")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(UserCreateResponse{
			Username: req.Username,
			Message:  "User created",
		})
	}
}

func HandleAuth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"authorized": "OK"})
}

func HandleGetProgress(storage *database.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := r.Header.Get("X-AUTH-USER")
		document := r.PathValue("document")

		if document == "" {
			http.Error(w, "Document ID is required", http.StatusBadRequest)
			return
		}

		progress, err := storage.GetProgress(username, document)
		if err != nil {
			slog.Error("failed to get progress", "username", username, "document", document, "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if progress == nil {
			slog.Warn("progress not found", "username", username, "document", document)
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		json.NewEncoder(w).Encode(progress)
	}
}

func HandleUpdateProgress(storage *database.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := r.Header.Get("X-AUTH-USER")

		var p models.Progress
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			slog.Error("failed to decode progress update", "username", username, "error", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if p.Document == "" {
			http.Error(w, "Document ID is required", http.StatusBadRequest)
			return
		}

		// Validate percentage
		if p.Percentage < 0 || p.Percentage > 1 {
			slog.Warn("invalid progress percentage", "username", username, "percentage", p.Percentage)
			http.Error(w, "Percentage must be between 0 and 1 inclusive", http.StatusBadRequest)
			return
		}

		// Set server-side timestamp if not provided or if we want to ensure server-side truth
		// KOReader might send a timestamp, but the server-side arrival time is often preferred for sync logic.
		p.Timestamp = time.Now().Unix()

		if err := storage.UpsertProgress(username, p); err != nil {
			slog.Error("failed to upsert progress", "username", username, "document", p.Document, "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		slog.Info("progress updated", "username", username, "document", p.Document)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "Progress updated"})
	}
}

func randomDelay() {
	n, err := rand.Int(rand.Reader, big.NewInt(500))
	if err != nil {
		time.Sleep(250 * time.Millisecond)
		return
	}
	time.Sleep(time.Duration(250+n.Int64()) * time.Millisecond)
}
