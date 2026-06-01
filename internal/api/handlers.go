package api

import (
	"crypto/rand"
	"encoding/json"
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
		log := GetLogger(r.Context())
		if cfg.DisableRegistration {
			log.Warn("registration attempt while disabled", "remote_addr", r.RemoteAddr, "source", "API")
			http.Error(w, "Registration is disabled", http.StatusForbidden)
			return
		}

		// Apply a random delay before any processing so that both new-user and
		// duplicate-user paths experience the same delay distribution, making
		// it harder for an attacker to infer whether an account exists.
		randomDelay()

		var req UserCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Error("failed to decode registration request", "error", err, "source", "API")
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Username == "" || req.Password == "" {
			http.Error(w, "Username and password are required", http.StatusBadRequest)
			return
		}

		// Check if user already exists. For duplicates, return a fake success
		// response (201 Created) without touching the stored password, so the
		// response is indistinguishable from a real registration.
		existingHash, err := storage.GetUserHash(req.Username)
		if err == nil && existingHash != "" {
			log.Info("registration attempt for existing user", "username", req.Username, "source", "API")
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
			log.Error("failed to hash password", "username", req.Username, "error", err, "source", "API")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if err := storage.CreateUser(req.Username, hash); err != nil {
			log.Error("failed to create user", "username", req.Username, "error", err, "source", "API")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		log.Info("user created successfully", "username", req.Username, "source", "API")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(UserCreateResponse{
			Username: req.Username,
			Message:  "User created",
		})
	}
}

func HandleAuth(w http.ResponseWriter, r *http.Request) {
	log := GetLogger(r.Context())
	username := GetUser(r.Context())
	if username == "" {
		username = r.Header.Get("X-AUTH-USER")
	}

	log.Info("auth successful", "username", username, "source", "API")
	log.Debug("auth response generated", "username", username, "source", "API")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"authorized": "OK"})
}

func HandleGetProgress(storage *database.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := GetLogger(r.Context())
		username := GetUser(r.Context())
		if username == "" {
			username = r.Header.Get("X-AUTH-USER")
		}
		document := r.PathValue("document")

		if document == "" {
			log.Warn("missing document id", "username", username, "path", r.URL.Path, "source", "API")
			http.Error(w, "Document ID is required", http.StatusBadRequest)
			return
		}

		progress, err := storage.GetProgress(username, document)
		if err != nil {
			log.Error("failed to get progress", "username", username, "document", document, "error", err, "source", "API")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if progress == nil {
			log.Warn("progress not found", "username", username, "document", document, "source", "API")
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		log.Info("progress retrieved", "username", username, "document", document, "percentage", progress.Percentage, "source", "API")
		log.Debug("progress lookup details", "username", username, "document", document, "percentage", progress.Percentage, "source", "API")
		json.NewEncoder(w).Encode(progress)
	}
}

func HandleUpdateProgress(storage *database.Storage, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := GetLogger(r.Context())
		username := GetUser(r.Context())
		if username == "" {
			username = r.Header.Get("X-AUTH-USER")
		}

		var p models.Progress
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			log.Error("failed to decode progress update", "username", username, "error", err, "source", "API")
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if p.Document == "" {
			log.Warn("missing document id", "username", username, "source", "API")
			http.Error(w, "Document ID is required", http.StatusBadRequest)
			return
		}

		// Validate percentage
		if p.Percentage < 0 || p.Percentage > 1 {
			log.Warn("invalid progress percentage", "username", username, "percentage", p.Percentage, "source", "API")
			http.Error(w, "Percentage must be between 0 and 1 inclusive", http.StatusBadRequest)
			return
		}

		// Set server-side timestamp if not provided.
		// If the client provides a timestamp, we respect it for multi-device sync logic.
		if p.Timestamp == 0 {
			p.Timestamp = time.Now().Unix()
		}

		if err := storage.UpsertProgress(username, p); err != nil {
			log.Error("failed to upsert progress", "username", username, "document", p.Document, "error", err, "source", "API")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Enforce storage cap
		if pruned, err := storage.EnforceStorageCap(cfg.DatabasePath, cfg.StorageCapMB); err != nil {
			log.Error("failed to enforce storage cap", "error", err, "source", "API")
		} else if pruned {
			log.Warn("storage cap enforced: oldest records pruned", "database_path", cfg.DatabasePath, "cap_mb", cfg.StorageCapMB, "source", "API")
		}

		log.Info("progress updated", "username", username, "document", p.Document, "percentage", p.Percentage, "timestamp", p.Timestamp, "source", "API")
		log.Debug("progress update details", "username", username, "document", p.Document, "percentage", p.Percentage, "timestamp", p.Timestamp, "source", "API")
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
