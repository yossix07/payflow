package idempotency

import (
	"bytes"
	"log/slog"
	"net/http"

	"github.com/payflow/payment-service/internal/repository"
)

// Middleware provides idempotency protection
type Middleware struct {
	repo repository.Repository
}

func NewMiddleware(repo repository.Repository) *Middleware {
	return &Middleware{repo: repo}
}

// Wrap wraps an HTTP handler with idempotency checking
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get idempotency key from header
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			http.Error(w, "Missing Idempotency-Key header", http.StatusBadRequest)
			return
		}

		// Check if we've seen this key before
		cachedResponse, statusCode, exists, err := m.repo.CheckIdempotency(r.Context(), key)
		if err != nil {
			slog.Error("Error checking idempotency", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if exists {
			// Return cached response
			slog.Info("Idempotency key already processed, returning cached response", "key", key)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			w.Write([]byte(cachedResponse))
			return
		}

		// Capture the response
		recorder := &responseRecorder{
			ResponseWriter: w,
			body:           &bytes.Buffer{},
			statusCode:     http.StatusOK,
		}

		// Call the next handler
		next.ServeHTTP(recorder, r)

		// Save the response for future requests
		if recorder.statusCode >= 200 && recorder.statusCode < 300 {
			if err := m.repo.SaveIdempotency(r.Context(), key, recorder.body.String(), recorder.statusCode); err != nil {
				slog.Error("Failed to save idempotency record", "error", err)
			}
		}
	})
}

// responseRecorder captures the HTTP response
type responseRecorder struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}
