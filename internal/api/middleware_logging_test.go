package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoggingMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	oldDefault := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldDefault)

	t.Run("Status 200", func(t *testing.T) {
		buf.Reset()
		mux := http.NewServeMux()
		mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		handler := LoggingMiddleware(mux)
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		logs := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
		var infoLog map[string]any
		found := false
		for _, b := range logs {
			var l map[string]any
			json.Unmarshal(b, &l)
			if l["level"] == "INFO" && l["msg"] == "request completed" {
				infoLog = l
				found = true
				break
			}
		}

		if !found {
			t.Fatal("expected INFO log not found")
		}
		if infoLog["status_code"] != float64(200) {
			t.Errorf("expected status 200, got %v", infoLog["status_code"])
		}
		if infoLog["request_id"] == nil {
			t.Error("expected request_id in logs")
		}
	})

	t.Run("Status 500 with error body", func(t *testing.T) {
		buf.Reset()
		mux := http.NewServeMux()
		mux.HandleFunc("POST /fail", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error occurred"))
		})

		handler := LoggingMiddleware(mux)
		req := httptest.NewRequest("POST", "/fail", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		logs := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
		var errorLog map[string]any
		found := false
		for _, b := range logs {
			var l map[string]any
			json.Unmarshal(b, &l)
			if l["level"] == "ERROR" {
				errorLog = l
				found = true
				break
			}
		}

		if !found {
			t.Fatal("expected ERROR log not found")
		}
		if errorLog["error_detail"] != "internal error occurred" {
			t.Errorf("expected error_detail 'internal error occurred', got %v", errorLog["error_detail"])
		}
	})

	t.Run("Request-scoped logger propagation", func(t *testing.T) {
		buf.Reset()
		mux := http.NewServeMux()
		mux.HandleFunc("GET /scoped", func(w http.ResponseWriter, r *http.Request) {
			l := GetLogger(r.Context())
			l.Info("inner log", "extra", "data")
			w.WriteHeader(http.StatusOK)
		})

		handler := LoggingMiddleware(mux)
		req := httptest.NewRequest("GET", "/scoped", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		logs := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
		var innerLog map[string]any
		found := false
		for _, b := range logs {
			var l map[string]any
			json.Unmarshal(b, &l)
			if l["msg"] == "inner log" {
				innerLog = l
				found = true
				break
			}
		}

		if !found {
			t.Fatal("expected inner log not found")
		}
		if innerLog["extra"] != "data" {
			t.Errorf("expected extra data, got %v", innerLog["extra"])
		}
		if innerLog["request_id"] == nil {
			t.Error("expected request_id in inner log")
		}
	})
}
