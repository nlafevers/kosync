package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	createCmd := flag.NewFlagSet("create-user", flag.ExitOnError)
	createUsername := createCmd.String("username", "", "Username")
	createPassword := createCmd.String("password", "", "Password")

	deleteCmd := flag.NewFlagSet("delete-user", flag.ExitOnError)
	deleteUsername := deleteCmd.String("username", "", "Username")

	switch os.Args[1] {
	case "create-user":
		createCmd.Parse(os.Args[2:])
		storage, _ := InitDB(config.DBPath)
		defer storage.Close()
		hash, _ := HashPassword(*createPassword)
		if err := storage.CreateUser(*createUsername, hash); err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Println("User created successfully")
		}
	case "delete-user":
		deleteCmd.Parse(os.Args[2:])
		storage, _ := InitDB(config.DBPath)
		defer storage.Close()
		if err := storage.DeleteUser(*deleteUsername); err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Println("User deleted successfully")
		}
	default:
		fmt.Println("Unknown command")
		os.Exit(1)
	}
}
