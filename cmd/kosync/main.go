package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"kosync/internal/api"
	"kosync/internal/config"
	"kosync/internal/database"
	"kosync/internal/logger"

	"golang.org/x/term"
	)
const appName = "kosync"

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}
	logger.New(cfg.LogLevel, cfg.JSONLog, cfg.LogPath)

	// Handle CLI commands
	if len(os.Args) > 1 {
		runCLI(cfg)
		return
	}

	slog.Info("Starting KOSYNC",
		"app_name", appName,
		"port", cfg.Port,
		"database_path", cfg.DatabasePath,
		"log_level", cfg.LogLevel,
		"json_log", cfg.JSONLog,
		"log_path", cfg.LogPath,
	)

	storage, err := database.InitDB(cfg.DatabasePath, true)
	if err != nil {
		slog.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer storage.Close()

	slog.Info("Database initialized",
		"database_path", cfg.DatabasePath,
		"migration_status", "success",
		"storage_cap_mb", cfg.StorageCapMB,
	)

	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("POST /users/create", api.HandleUserCreate(storage, cfg))

	// Protected routes
	protected := http.NewServeMux()
	protected.HandleFunc("GET /users/auth", api.HandleAuth)
	protected.HandleFunc("GET /syncs/progress/{document}", api.HandleGetProgress(storage))
	protected.HandleFunc("PUT /syncs/progress", api.HandleUpdateProgress(storage, cfg))

	// Middleware chaining
	var handler http.Handler = protected
	handler = api.AuthMiddleware(storage, handler)
	handler = api.AcceptMiddleware(handler)
	handler = api.ContentTypeMiddleware(handler)

	mux.Handle("/", handler)
	slog.Info("server listening", "port", cfg.Port)

	// Graceful shutdown
	server := &http.Server{Addr: fmt.Sprintf(":%d", cfg.Port), Handler: api.LoggingMiddleware(mux)}
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		sig := <-sigChan
		slog.Info("Shutdown signal received", "signal", sig.String())
		slog.Info("Shutting down server...")

		if err := server.Shutdown(context.Background()); err != nil {
			slog.Error("Server shutdown failed", "error", err)
		} else {
			slog.Info("Server exited cleanly")
		}
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
	slog.Info("server exited cleanly")
}

func runCLI(cfg *config.Config) {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]

	switch command {
	case "create-user":
	        if len(os.Args) < 3 {
	                fmt.Printf("Usage: %s %s <username> [--password-stdin]\n", appName, command)
	                os.Exit(1)
	        }
	        username := os.Args[2]
	        password, err := passwordFromArgs(os.Args[3:], os.Stdin, os.Stdout)
	        if err != nil {
	                logger.LogCLIFailure(nil, command, username, "failed to read password: "+err.Error())
	                fmt.Printf("Failed to read password: %v\n", err)
	                os.Exit(1)
	        }
	        createUser(cfg, username, password)
	case "delete-user":
	        if len(os.Args) < 3 {
	                fmt.Printf("Usage: %s %s <username>\n", appName, command)
	                os.Exit(1)
	        }
	        username := os.Args[2]
	        deleteUser(cfg, username)
	case "change-password":
	        if len(os.Args) < 3 {
	                fmt.Printf("Usage: %s %s <username> [--password-stdin]\n", appName, command)
	                os.Exit(1)
	        }
	        username := os.Args[2]
	        password, err := passwordFromArgs(os.Args[3:], os.Stdin, os.Stdout)
	        if err != nil {
	                logger.LogCLIFailure(nil, command, username, "failed to read password: "+err.Error())
	                fmt.Printf("Failed to read password: %v\n", err)
	                os.Exit(1)
	        }
	        changePassword(cfg, username, password)
	default:		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Printf("  %s                          Run the server\n", appName)
	fmt.Printf("  %s create-user <username>   Create a new user\n", appName)
	fmt.Printf("  %s delete-user <username>   Delete a user\n", appName)
	fmt.Printf("  %s change-password <user>   Change a user's password\n", appName)
	fmt.Println("\nOptions for user commands:")
	fmt.Println("  --password-stdin                Read password from stdin")
}

func createUser(cfg *config.Config, username, password string) {
        operation := "create-user"
        storage := openCLIStorage(cfg)
        defer storage.Close()

        hash, err := api.HashPassword(password)
        if err != nil {
                logger.LogCLIFailure(nil, operation, username, "failed to hash password: "+err.Error())
                fmt.Printf("Failed to hash password: %v\n", err)
                os.Exit(1)
        }
        if err := storage.CreateUserIfNotExists(username, hash); err != nil {
                if err.Error() == "user already exists" {
                        logger.LogCLIFailure(nil, operation, username, "user already exists")
                        fmt.Printf("Error: User '%s' already exists\n", username)
                        os.Exit(1)
                }
                logger.LogCLIFailure(nil, operation, username, "failed to save user: "+err.Error())
                fmt.Printf("Failed to save user: %v\n", err)
                os.Exit(1)
        }
        logger.LogCLISuccess(nil, operation, username)
        fmt.Printf("User '%s' created successfully.\n", username)
}
func deleteUser(cfg *config.Config, username string) {
        operation := "delete-user"
        storage := openCLIStorage(cfg)
        defer storage.Close()

        if err := storage.DeleteUser(username); err != nil {
                logger.LogCLIFailure(nil, operation, username, "failed to delete user: "+err.Error())
                fmt.Printf("Failed to delete user: %v\n", err)
                os.Exit(1)
        }
        logger.LogCLISuccess(nil, operation, username)
        fmt.Printf("User '%s' deleted successfully.\n", username)
}
func changePassword(cfg *config.Config, username, password string) {
        operation := "change-password"
        storage := openCLIStorage(cfg)
        defer storage.Close()

        hash, err := api.HashPassword(password)
        if err != nil {
                logger.LogCLIFailure(nil, operation, username, "failed to hash password: "+err.Error())
                fmt.Printf("Failed to hash password: %v\n", err)
                os.Exit(1)
        }
        if err := storage.UpdateUserPassword(username, hash); err != nil {
                logger.LogCLIFailure(nil, operation, username, "failed to update user password: "+err.Error())
                fmt.Printf("Failed to update password: %v\n", err)
                os.Exit(1)
        }
        logger.LogCLISuccess(nil, operation, username)
        fmt.Printf("Password for user '%s' updated successfully.\n", username)
}
func openCLIStorage(cfg *config.Config) *database.Storage {
	storage, err := database.InitDB(cfg.DatabasePath, true)
	if err != nil {
		fmt.Printf("Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	return storage
}

func passwordFromArgs(args []string, stdin io.Reader, stdout io.Writer) (string, error) {
	switch len(args) {
	case 0:
		return readPasswordInteractively(stdout)
	case 1:
		if args[0] != "--password-stdin" {
			return "", errors.New("password arguments are not supported; use interactive prompt or --password-stdin")
		}
		passwordBytes, err := io.ReadAll(stdin)
		if err != nil {
			return "", err
		}
		password := strings.TrimRight(string(passwordBytes), "\r\n")
		if password == "" {
			return "", errors.New("password cannot be empty")
		}
		return password, nil
	default:
		return "", errors.New("too many arguments")
	}
}

func readPasswordInteractively(stdout io.Writer) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("stdin is not a terminal; use --password-stdin for automation")
	}

	fmt.Fprint(stdout, "Password: ")
	first, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(stdout)
	if err != nil {
		return "", err
	}

	fmt.Fprint(stdout, "Confirm password: ")
	second, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(stdout)
	if err != nil {
		return "", err
	}

	if string(first) == "" {
		return "", errors.New("password cannot be empty")
	}
	if string(first) != string(second) {
		return "", errors.New("passwords do not match")
	}
	return string(first), nil
}
