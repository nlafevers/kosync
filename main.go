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

	"golang.org/x/term"
)

func main() {
	config := LoadConfig()

	// Handle CLI commands
	if len(os.Args) > 1 {
		runCLI(config)
		return
	}

	InitLogger(config.LogLevel)

	slog.Info("KOSYNC starting",
		"port", config.Port,
		"db_path", config.DBPath,
		"log_level", config.LogLevel,
	)

	storage, err := InitDB(config.DBPath)
	if err != nil {
		slog.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer storage.Close()

	slog.Info("database initialized successfully")

	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("POST /users/create", handleUserCreate(storage, &config))

	// Protected routes
	protected := http.NewServeMux()
	protected.HandleFunc("GET /users/auth", handleAuth)
	protected.HandleFunc("GET /syncs/progress/{document}", handleGetProgress(storage))
	protected.HandleFunc("PUT /syncs/progress", handleUpdateProgress(storage))

	// Middleware chaining
	var handler http.Handler = protected
	handler = AuthMiddleware(storage, handler)
	handler = AcceptMiddleware(handler)
	handler = ContentTypeMiddleware(handler)

	mux.Handle("/", handler)

	slog.Info("server listening", "port", config.Port)

	// Graceful shutdown
	server := &http.Server{Addr: ":" + config.Port, Handler: mux}
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		slog.Info("shutdown signal received")
		if err := server.Shutdown(context.Background()); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
	slog.Info("server exited cleanly")
}

func runCLI(config Config) {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]

	switch command {
	case "create-user", "delete-user", "change-password":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: kosync %s <username> [--password-stdin]\n", command)
			os.Exit(1)
		}
		username := os.Args[2]

		storage, err := InitDB(config.DBPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to initialize database: %v\n", err)
			os.Exit(1)
		}
		defer storage.Close()

		switch command {
		case "create-user":
			password, err := passwordFromArgs(os.Args[3:], os.Stdin, os.Stdout)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			hash, err := HashPassword(password)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to hash password: %v\n", err)
				os.Exit(1)
			}
			if err := storage.CreateUser(username, hash); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("User '%s' created successfully\n", username)

		case "delete-user":
			if err := storage.DeleteUser(username); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("User '%s' deleted successfully\n", username)

		case "change-password":
			password, err := passwordFromArgs(os.Args[3:], os.Stdin, os.Stdout)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			hash, err := HashPassword(password)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to hash password: %v\n", err)
				os.Exit(1)
			}
			if err := storage.UpdateUserPassword(username, hash); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Password for user '%s' updated successfully\n", username)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  kosync                          Run the server")
	fmt.Println("  kosync create-user <username>   Create a new user")
	fmt.Println("  kosync delete-user <username>   Delete a user")
	fmt.Println("  kosync change-password <user>   Change a user's password")
	fmt.Println("\nOptions for user commands:")
	fmt.Println("  --password-stdin                Read password from stdin")
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
