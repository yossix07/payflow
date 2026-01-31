package main

import (
	"encoding/json"
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

	// K8s Readiness Probe
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Println("Starting Platform Dashboard on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
