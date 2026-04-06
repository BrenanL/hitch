package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/BrenanL/hitch/internal/state"
)

// Daemon is the background monitoring service that aggregates session data
// from SQLite, proxy logs, and JSONL transcripts.
type Daemon struct {
	port       int
	db         *state.DB
	pidPath    string
	httpServer *http.Server
	startTime  time.Time
	pollCount  atomic.Int64

	// Tracker holds live session state (in-memory, rebuilt on restart).
	Tracker *SessionTracker
	SSEHub  *SSEHub
	Alerts  *AlertEvaluator
}

// New creates a Daemon with the given port and database.
func New(port int, db *state.DB) *Daemon {
	return &Daemon{
		port:    port,
		db:      db,
		pidPath: DefaultPIDPath(),
		Tracker: NewSessionTracker(),
		SSEHub:  NewSSEHub(),
		Alerts:  NewAlertEvaluator(DefaultAlertConfig(), nil),
	}
}

// NewWithPIDPath creates a Daemon with a custom PID path (for testing).
func NewWithPIDPath(port int, db *state.DB, pidPath string) *Daemon {
	return &Daemon{
		port:    port,
		db:      db,
		pidPath: pidPath,
		Tracker: NewSessionTracker(),
		SSEHub:  NewSSEHub(),
		Alerts:  NewAlertEvaluator(DefaultAlertConfig(), nil),
	}
}

// Start runs the daemon. If foreground is false, it re-execs itself with
// --foreground and returns immediately (the parent exits). If foreground is
// true, it runs the HTTP server and blocks until SIGTERM/SIGINT.
func (d *Daemon) Start(foreground bool) error {
	if !foreground {
		return d.forkToBackground()
	}
	return d.run()
}

// forkToBackground re-execs the current binary with --foreground flag.
func (d *Daemon) forkToBackground() error {
	// Check if already running
	existing, err := ReadPID(d.pidPath)
	if err == nil && IsRunning(existing) {
		return fmt.Errorf("daemon already running (PID %d, port %d)", existing.PID, existing.Port)
	}

	args := []string{"daemon", "start", "--foreground", "--port", fmt.Sprintf("%d", d.port)}
	cmd := exec.Command(os.Args[0], args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	// Detach from parent process group
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("forking daemon: %w", err)
	}

	fmt.Printf("Hitch daemon started (PID %d, port %d)\n", cmd.Process.Pid, d.port)
	return nil
}

// run is the main daemon loop that runs in the foreground.
func (d *Daemon) run() error {
	// Check if already running
	existing, err := ReadPID(d.pidPath)
	if err == nil && IsRunning(existing) {
		return fmt.Errorf("daemon already running (PID %d, port %d)", existing.PID, existing.Port)
	}

	// Write PID file
	d.startTime = time.Now()
	if err := WritePID(d.pidPath, PIDInfo{
		PID:       os.Getpid(),
		Port:      d.port,
		StartedAt: d.startTime.Format(time.RFC3339),
	}); err != nil {
		return fmt.Errorf("writing PID file: %w", err)
	}
	defer RemovePID(d.pidPath)

	// Set up HTTP server
	mux := http.NewServeMux()
	d.registerRoutes(mux)
	d.httpServer = &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", d.port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // SSE streams have no write timeout
		IdleTimeout:  120 * time.Second,
	}

	// Signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start HTTP server
	errCh := make(chan error, 1)
	go func() {
		log.Printf("hitch daemon listening on 127.0.0.1:%d", d.port)
		errCh <- d.httpServer.ListenAndServe()
	}()

	// Start aggregator poll loop
	stopAgg := make(chan struct{})
	go d.aggregatorLoop(stopAgg)

	// Wait for signal or error
	select {
	case err := <-errCh:
		close(stopAgg)
		if err != http.ErrServerClosed {
			return err
		}
	case sig := <-sigCh:
		close(stopAgg)
		log.Printf("received %v, shutting down", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		d.httpServer.Shutdown(ctx)
	}

	return nil
}

// Stop sends SIGTERM to a running daemon by reading its PID file.
func Stop(pidPath string) error {
	info, err := ReadPID(pidPath)
	if err != nil {
		return fmt.Errorf("daemon not running: %w", err)
	}

	if !IsRunning(info) {
		RemovePID(pidPath)
		return fmt.Errorf("daemon not running (stale PID %d)", info.PID)
	}

	proc, err := os.FindProcess(info.PID)
	if err != nil {
		return fmt.Errorf("finding process %d: %w", info.PID, err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("sending SIGTERM to %d: %w", info.PID, err)
	}

	// Wait up to 5 seconds for clean exit
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !IsRunning(info) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Force kill if still running
	if IsRunning(info) {
		proc.Signal(syscall.SIGKILL)
		time.Sleep(100 * time.Millisecond)
	}

	RemovePID(pidPath)
	return nil
}

// Status returns the current daemon status by reading the PID file and
// querying the /health endpoint.
func Status(pidPath string, port int) (*StatusInfo, error) {
	info, err := ReadPID(pidPath)
	if err != nil {
		return &StatusInfo{Running: false}, nil
	}

	if !IsRunning(info) {
		return &StatusInfo{Running: false, StalePID: info.PID}, nil
	}

	// Try health endpoint
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", info.Port))
	if err != nil {
		return &StatusInfo{
			Running:     true,
			PID:         info.PID,
			Port:        info.Port,
			HealthError: err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	var health HealthResponse
	json.NewDecoder(resp.Body).Decode(&health)

	return &StatusInfo{
		Running: true,
		PID:     info.PID,
		Port:    info.Port,
		Health:  &health,
	}, nil
}

// StatusInfo is returned by Status.
type StatusInfo struct {
	Running     bool
	PID         int
	Port        int
	StalePID    int
	HealthError string
	Health      *HealthResponse
}

// registerRoutes sets up all HTTP endpoints.
func (d *Daemon) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", d.handleHealth)
	mux.HandleFunc("/api/sessions", d.handleSessions)
	mux.HandleFunc("/api/sessions/", d.handleSessionByID)
	mux.HandleFunc("/api/stats", d.handleStats)
	mux.HandleFunc("/api/alerts", d.handleAlerts)
}

// aggregatorLoop polls data sources on a fixed interval.
func (d *Daemon) aggregatorLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			d.pollOnce()
		}
	}
}

// pollOnce runs one poll cycle against all data sources.
func (d *Daemon) pollOnce() {
	d.pollCount.Add(1)

	if d.db == nil {
		return
	}

	d.pollRequests()
	d.pollEvents()

	// Mark stale sessions as inactive and prune old ones
	d.Tracker.MarkInactive(5 * time.Minute)
	d.Tracker.Prune(24 * time.Hour)

	// Evaluate alert conditions
	d.Alerts.Evaluate(d.Tracker)
}

// pollRequests reads new API request rows from SQLite.
func (d *Daemon) pollRequests() {
	requests, err := d.db.QueryRecentRequests(50, "")
	if err != nil {
		log.Printf("[daemon] poll requests error: %v", err)
		return
	}

	lastSeen := d.Tracker.LastRequestID()
	for _, req := range requests {
		if req.SessionID == "" || req.ID <= lastSeen {
			continue
		}
		d.Tracker.UpdateFromRequest(req)

		// Broadcast to SSE subscribers
		d.SSEHub.Broadcast(req.SessionID, DaemonEvent{
			Timestamp:   time.Now().Format(time.RFC3339),
			Source:      "proxy",
			EventType:   "api_request",
			SessionID:   req.SessionID,
			Description: fmt.Sprintf("%s %d in/%d out", req.Model, req.InputTokens, req.OutputTokens),
		})
	}
}

// pollEvents reads new hook events from SQLite.
func (d *Daemon) pollEvents() {
	events, err := d.db.EventQuery(state.EventFilter{Limit: 50})
	if err != nil {
		log.Printf("[daemon] poll events error: %v", err)
		return
	}

	lastSeen := d.Tracker.LastEventID()
	for _, evt := range events {
		if evt.ID <= lastSeen {
			continue
		}
		d.Tracker.UpdateFromEvent(evt)

		// Fire event-triggered alerts (e.g. StopFailure -> rate_limit_hit)
		d.Alerts.FireFromEvent(evt.SessionID, evt.HookEvent, evt.ActionTaken)

		// Broadcast to SSE subscribers
		d.SSEHub.Broadcast(evt.SessionID, DaemonEvent{
			Timestamp:   evt.Timestamp,
			Source:      "hooks",
			EventType:   evt.HookEvent,
			SessionID:   evt.SessionID,
			Description: formatEventDescription(evt),
		})
	}
}
