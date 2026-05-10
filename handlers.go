package main

import (
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"math/big"
	"net/http"
	"time"
)

type UserCreateRequest struct {
	Username string `json:"username"`
	Password string `json:"password"` // This is the MD5 from the client
}

type UserCreateResponse struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

func handleUserCreate(storage *Storage, config *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if config.DisableRegistration {
			slog.Warn("registration attempt while disabled", "remote_addr", r.RemoteAddr)
			http.Error(w, "Registration is disabled", http.StatusForbidden)
			return
		}

		var req UserCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("failed to decode registration request", "error", err)
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
			slog.Warn("registration attempt for existing user", "username", req.Username)
			// Random delay to prevent timing attacks
			randomDelay()
			http.Error(w, "User already exists", http.StatusConflict)
			return
		}

		// Hash the password (which is already an MD5 from the client) using Bcrypt
		hash, err := HashPassword(req.Password)
		if err != nil {
			slog.Error("failed to hash password", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if err := storage.CreateUser(req.Username, hash); err != nil {
			slog.Error("failed to create user", "username", req.Username, "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		slog.Info("user created successfully", "username", req.Username)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(UserCreateResponse{
			Username: req.Username,
			Message:  "User created",
		})
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
