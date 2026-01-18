package functions

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/markb/sblite/internal/log"
)

// RuntimeConfig holds configuration for the edge runtime process.
type RuntimeConfig struct {
	// BinaryPath is the path to the edge-runtime binary (auto-downloaded if empty)
	BinaryPath string
	// FunctionsDir is the directory containing functions
	FunctionsDir string
	// Port is the port edge-runtime will listen on
	Port int
	// SblitePort is the sblite server port (for SUPABASE_URL env var)
	SblitePort int
	// AnonKey is the anon API key
	AnonKey string
	// ServiceKey is the service_role API key
	ServiceKey string
	// DBPath is the database path
	DBPath string
	// Secrets contains environment variables to inject from secrets store
	Secrets map[string]string
}

// RuntimeManager manages the edge runtime subprocess.
type RuntimeManager struct {
	config       RuntimeConfig
	binaryPath   string
	process      *exec.Cmd
	processLock  sync.Mutex
	healthy      bool
	healthTicker *time.Ticker
	stopCh       chan struct{}
}

// NewRuntimeManager creates a new runtime manager.
func NewRuntimeManager(config RuntimeConfig) *RuntimeManager {
	if config.Port == 0 {
		config.Port = 8081
	}
	if config.FunctionsDir == "" {
		config.FunctionsDir = "./functions"
	}

	return &RuntimeManager{
		config: config,
	}
}

// Start starts the edge runtime process.
func (rm *RuntimeManager) Start(ctx context.Context) error {
	rm.processLock.Lock()
	defer rm.processLock.Unlock()

	if rm.process != nil {
		return fmt.Errorf("runtime already started")
	}

	// Ensure binary exists (download if needed)
	binaryPath, err := rm.ensureBinary()
	if err != nil {
		return fmt.Errorf("failed to ensure edge runtime binary: %w", err)
	}
	rm.binaryPath = binaryPath

	// Resolve absolute path for functions directory
	functionsDir, err := filepath.Abs(rm.config.FunctionsDir)
	if err != nil {
		return fmt.Errorf("failed to resolve functions directory: %w", err)
	}

	// Build command arguments
	// Edge runtime expects: edge-runtime start --main-service <path> --port <port>
	args := []string{
		"start",
		"--main-service", functionsDir,
		"--port", fmt.Sprintf("%d", rm.config.Port),
	}

	log.Info("starting edge runtime",
		"binary", rm.binaryPath,
		"functions_dir", functionsDir,
		"port", rm.config.Port,
	)

	rm.process = exec.CommandContext(ctx, rm.binaryPath, args...)

	// Set environment variables
	rm.process.Env = rm.buildEnv()

	// Capture output for logging
	rm.process.Stdout = &runtimeLogWriter{prefix: "[edge-runtime]", level: "info"}
	rm.process.Stderr = &runtimeLogWriter{prefix: "[edge-runtime]", level: "error"}

	// Start the process
	if err := rm.process.Start(); err != nil {
		rm.process = nil
		return fmt.Errorf("failed to start edge runtime: %w", err)
	}

	// Start health check goroutine
	rm.stopCh = make(chan struct{})
	rm.healthTicker = time.NewTicker(5 * time.Second)
	go rm.healthCheckLoop()

	// Wait for runtime to be ready
	if err := rm.waitForReady(ctx, 30*time.Second); err != nil {
		rm.Stop()
		return fmt.Errorf("edge runtime failed to start: %w", err)
	}

	rm.healthy = true
	log.Info("edge runtime started successfully", "port", rm.config.Port)
	return nil
}

// Stop stops the edge runtime process.
func (rm *RuntimeManager) Stop() error {
	rm.processLock.Lock()
	defer rm.processLock.Unlock()

	if rm.process == nil {
		return nil
	}

	// Stop health check
	if rm.stopCh != nil {
		close(rm.stopCh)
		rm.stopCh = nil
	}
	if rm.healthTicker != nil {
		rm.healthTicker.Stop()
		rm.healthTicker = nil
	}

	log.Info("stopping edge runtime")

	// Send SIGTERM for graceful shutdown
	if rm.process.Process != nil {
		rm.process.Process.Signal(syscall.SIGTERM)
	}

	// Wait with timeout
	done := make(chan error, 1)
	go func() {
		done <- rm.process.Wait()
	}()

	select {
	case <-done:
		// Process exited cleanly
	case <-time.After(5 * time.Second):
		// Force kill
		log.Warn("edge runtime did not stop gracefully, sending SIGKILL")
		if rm.process.Process != nil {
			rm.process.Process.Kill()
		}
		<-done
	}

	rm.process = nil
	rm.healthy = false
	log.Info("edge runtime stopped")
	return nil
}

// Restart restarts the edge runtime process.
func (rm *RuntimeManager) Restart(ctx context.Context) error {
	if err := rm.Stop(); err != nil {
		return err
	}
	return rm.Start(ctx)
}

// IsHealthy returns true if the runtime is responding to health checks.
func (rm *RuntimeManager) IsHealthy() bool {
	rm.processLock.Lock()
	defer rm.processLock.Unlock()
	return rm.healthy
}

// Port returns the port the runtime is listening on.
func (rm *RuntimeManager) Port() int {
	return rm.config.Port
}

// ensureBinary ensures the edge runtime binary is available.
func (rm *RuntimeManager) ensureBinary() (string, error) {
	// If explicit path is provided, use it
	if rm.config.BinaryPath != "" {
		if _, err := os.Stat(rm.config.BinaryPath); err != nil {
			return "", fmt.Errorf("edge runtime binary not found at %s: %w", rm.config.BinaryPath, err)
		}
		return rm.config.BinaryPath, nil
	}

	// Check common locations
	commonPaths := []string{
		"./bin/edge-runtime",
		"./edge-runtime",
		"/usr/local/bin/edge-runtime",
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Check if in PATH
	if path, err := exec.LookPath("edge-runtime"); err == nil {
		return path, nil
	}

	// Check if platform supports automatic download
	if !IsSupported() {
		return "", UnsupportedPlatformError()
	}

	// Try to download
	downloader := NewDownloader(DefaultDownloadDir())
	path, err := downloader.EnsureBinary()
	if err != nil {
		return "", fmt.Errorf("edge runtime not found and download failed: %w", err)
	}

	return path, nil
}

// buildEnv builds the environment variables for the edge runtime process.
func (rm *RuntimeManager) buildEnv() []string {
	env := os.Environ()

	// Add sblite-specific environment variables
	sbliteURL := fmt.Sprintf("http://127.0.0.1:%d", rm.config.SblitePort)
	if rm.config.SblitePort == 0 {
		sbliteURL = "http://127.0.0.1:8080"
	}

	env = append(env,
		fmt.Sprintf("SUPABASE_URL=%s", sbliteURL),
		fmt.Sprintf("SUPABASE_ANON_KEY=%s", rm.config.AnonKey),
		fmt.Sprintf("SUPABASE_SERVICE_ROLE_KEY=%s", rm.config.ServiceKey),
	)

	// Add DB path if available (for reference, not direct access)
	if rm.config.DBPath != "" {
		env = append(env, fmt.Sprintf("SUPABASE_DB_URL=sqlite://%s", rm.config.DBPath))
	}

	// Add secrets from the store (injected as environment variables)
	for name, value := range rm.config.Secrets {
		env = append(env, fmt.Sprintf("%s=%s", name, value))
	}

	return env
}

// UpdateSecrets updates the secrets configuration. This requires a restart to take effect.
func (rm *RuntimeManager) UpdateSecrets(secrets map[string]string) {
	rm.processLock.Lock()
	defer rm.processLock.Unlock()
	rm.config.Secrets = secrets
}

// waitForReady waits for the edge runtime to be ready.
func (rm *RuntimeManager) waitForReady(ctx context.Context, timeout time.Duration) error {
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", rm.config.Port)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp, err := http.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for edge runtime to be ready")
}

// healthCheckLoop runs periodic health checks.
func (rm *RuntimeManager) healthCheckLoop() {
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", rm.config.Port)

	for {
		select {
		case <-rm.stopCh:
			return
		case <-rm.healthTicker.C:
			resp, err := http.Get(healthURL)
			healthy := err == nil && resp != nil && resp.StatusCode == http.StatusOK
			if resp != nil {
				resp.Body.Close()
			}

			rm.processLock.Lock()
			wasHealthy := rm.healthy
			rm.healthy = healthy
			rm.processLock.Unlock()

			if wasHealthy && !healthy {
				log.Warn("edge runtime health check failed")
			} else if !wasHealthy && healthy {
				log.Info("edge runtime health restored")
			}
		}
	}
}

// runtimeLogWriter is an io.Writer that logs edge runtime output.
type runtimeLogWriter struct {
	prefix string
	level  string
}

func (w *runtimeLogWriter) Write(p []byte) (n int, err error) {
	msg := string(p)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	if msg == "" {
		return len(p), nil
	}

	switch w.level {
	case "error":
		log.Error(msg, "source", w.prefix)
	default:
		log.Debug(msg, "source", w.prefix)
	}
	return len(p), nil
}
