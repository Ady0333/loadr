package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/joho/godotenv"

	"github.com/Ady0333/loadr/ai"
	"github.com/Ady0333/loadr/engine"
	"github.com/Ady0333/loadr/ws"
)

const (
	defaultPort = "8080"
	staticDir   = "../frontend/dist"
	allowOrigin = "http://localhost:5173"
)

// runRequest is the JSON body accepted by POST /api/run. Duration is taken in
// whole seconds so the API stays language-agnostic (Go's time.Duration
// serializes as raw nanoseconds, which is awkward for a JS client to produce).
type runRequest struct {
	URL             string            `json:"url"`
	Method          string            `json:"method"`
	Headers         map[string]string `json:"headers"`
	Body            string            `json:"body"`
	Concurrency     int               `json:"concurrency"`
	DurationSeconds int               `json:"durationSeconds"`
}

type server struct {
	hub *ws.Hub
	// running guards against overlapping load tests; the hub broadcasts a
	// single stream, so we allow one run at a time.
	running int32
}

func main() {
	// Load .env if present; missing file is fine.
	_ = godotenv.Load()

	hub := ws.NewHub()
	srv := &server{hub: hub}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/run", srv.handleRun)
	mux.HandleFunc("/api/analyze", srv.handleAnalyze)
	mux.HandleFunc("/ws", hub.ServeWS)
	mux.Handle("/", http.FileServer(http.Dir(staticDir)))

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}
	listenAddr := ":" + port

	log.Printf("loadr listening on %s", listenAddr)
	if err := http.ListenAndServe(listenAddr, withCORS(mux)); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func (s *server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req runRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	cfg, err := req.toConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Reject a new run while one is already in flight.
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		http.Error(w, "a load test is already running", http.StatusConflict)
		return
	}

	go s.runTest(cfg)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{
		"status":          "started",
		"url":             cfg.URL,
		"concurrency":     cfg.Concurrency,
		"durationSeconds": int(cfg.Duration.Seconds()),
	})
}

func (s *server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var snapshots []engine.MetricsSnapshot
	if err := json.NewDecoder(r.Body).Decode(&snapshots); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	report, err := ai.AnalyzeMetrics(snapshots)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(report))
}

// runTest executes a load test and streams each MetricsSnapshot to all
// connected WebSocket clients as JSON.
func (s *server) runTest(cfg engine.RunConfig) {
	defer atomic.StoreInt32(&s.running, 0)

	snapshots := engine.Collect(engine.Run(cfg))
	for snap := range snapshots {
		payload, err := json.Marshal(snap)
		if err != nil {
			log.Printf("run: marshal snapshot: %v", err)
			continue
		}
		s.hub.Broadcast(payload)
	}
}

// toConfig validates the request and converts it into an engine.RunConfig.
func (r runRequest) toConfig() (engine.RunConfig, error) {
	if r.URL == "" {
		return engine.RunConfig{}, errBadRequest("url is required")
	}
	if r.Concurrency <= 0 {
		return engine.RunConfig{}, errBadRequest("concurrency must be > 0")
	}
	if r.DurationSeconds <= 0 {
		return engine.RunConfig{}, errBadRequest("durationSeconds must be > 0")
	}

	method := r.Method
	if method == "" {
		method = http.MethodGet
	}

	return engine.RunConfig{
		URL:         r.URL,
		Method:      method,
		Headers:     r.Headers,
		Body:        r.Body,
		Concurrency: r.Concurrency,
		Duration:    time.Duration(r.DurationSeconds) * time.Second,
	}, nil
}

type errBadRequest string

func (e errBadRequest) Error() string { return string(e) }

// withCORS allows the Vite dev server to call the API and answers preflight
// requests.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
