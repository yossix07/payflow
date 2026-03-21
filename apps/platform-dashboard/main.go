package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type StatusResponse struct {
	Service   string    `json:"service"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Hostname  string    `json:"hostname"`
}

func triggerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	paymentURL := os.Getenv("PAYMENT_SERVICE_URL")
	if paymentURL == "" {
		paymentURL = "http://payment-service:8080"
	}

	proxyReq, _ := http.NewRequest("POST", paymentURL+"/payments", r.Body)
	proxyReq.Header.Set("Content-Type", "application/json")
	if key := r.Header.Get("Idempotency-Key"); key != "" {
		proxyReq.Header.Set("Idempotency-Key", key)
	}
	resp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		http.Error(w, "Failed to reach payment service: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func eventsHandler(w http.ResponseWriter, r *http.Request) {
	notifURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if notifURL == "" {
		notifURL = "http://notification-service:8080"
	}

	resp, err := http.Get(notifURL + "/events")
	if err != nil {
		http.Error(w, "Failed to reach notification service", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			flusher.Flush()
		}
		if err != nil {
			break
		}
	}
}

func main() {
	// Serve static files from the "static" directory
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	// API endpoint for status/health
	http.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		hostname, _ := os.Hostname()
		resp := StatusResponse{
			Service:   "Platform Dashboard",
			Status:    "Operational",
			Timestamp: time.Now(),
			Hostname:  hostname,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// Proxy endpoints
	http.HandleFunc("/api/trigger", triggerHandler)
	http.HandleFunc("/api/events", eventsHandler)

	// K8s Readiness Probe
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Println("Starting Platform Dashboard on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
