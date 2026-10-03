package runtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const defaultDaemonURL = "http://127.0.0.1:11435"

type Supervisor struct {
	paths   Paths
	dataDir string

	mu      sync.Mutex
	cmd     *exec.Cmd
	logFile *os.File
	baseURL string
	owned   bool
}

func NewSupervisor(paths Paths, dataDir string) *Supervisor {
	return &Supervisor{paths: paths, dataDir: dataDir}
}

func (s *Supervisor) Start(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.baseURL != "" {
		return s.baseURL, nil
	}
	baseURL := os.Getenv("MODELCTL_URL")
	if baseURL == "" {
		baseURL = defaultDaemonURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if !isLoopbackURL(baseURL) {
		s.baseURL = baseURL
		return baseURL, nil
	}
	if daemonReady(ctx, baseURL) {
		s.baseURL = baseURL
		return baseURL, nil
	}
	if s.paths.NodeBinary == "" || s.paths.ControlPlaneRoot == "" {
		return "", errors.New("packaged runtime paths are incomplete")
	}
	if _, err := os.Stat(s.paths.NodeBinary); err != nil {
		return "", fmt.Errorf("packaged Node runtime is unavailable: %w", err)
	}
	if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
		return "", fmt.Errorf("create Modelctl data directory: %w", err)
	}
	if s.paths.LogDir == "" {
		s.paths.LogDir = filepath.Join(s.dataDir, "logs")
	}
	if err := os.MkdirAll(s.paths.LogDir, 0o755); err != nil {
		return "", fmt.Errorf("create runtime log directory: %w", err)
	}
	logPath := filepath.Join(s.paths.LogDir, "daemon.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return "", fmt.Errorf("open daemon log: %w", err)
	}
	args := []string{filepath.Join(s.paths.ControlPlaneRoot, "bin", "modelctl.js"), "daemon"}
	cmd := exec.Command(s.paths.NodeBinary, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	env := append([]string{}, os.Environ()...)
	env = setEnv(env, "MODELCTL_DATA_DIR", s.dataDir)
	env = setEnv(env, "MODELCTL_RUNTIME_ROOT", s.paths.Root)
	env = setEnv(env, "MODELCTL_URL", baseURL)
	if s.paths.PythonBinary != "" {
		env = setEnv(env, "MODELCTL_PYTHON", s.paths.PythonBinary)
	}
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return "", fmt.Errorf("start packaged Modelctl daemon: %w", err)
	}
	s.cmd = cmd
	s.logFile = logFile
	s.owned = true
	readyCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	for {
		if daemonReady(readyCtx, baseURL) {
			s.baseURL = baseURL
			return baseURL, nil
		}
		select {
		case <-readyCtx.Done():
			_ = s.stopLocked()
			return "", fmt.Errorf("packaged Modelctl daemon did not become ready at %s: %w", baseURL, readyCtx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (s *Supervisor) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopLocked()
}

func (s *Supervisor) stopLocked() error {
	if !s.owned || s.cmd == nil {
		if s.logFile != nil {
			_ = s.logFile.Close()
			s.logFile = nil
		}
		return nil
	}
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	err := s.cmd.Wait()
	if s.cmd.Process != nil {
		// A process terminated by supervisor.Close is an expected shutdown.
		err = nil
	}
	if s.logFile != nil {
		_ = s.logFile.Close()
		s.logFile = nil
	}
	s.cmd = nil
	s.owned = false
	return err
}

func daemonReady(ctx context.Context, baseURL string) bool {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 500 * time.Millisecond}
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode >= 200 && response.StatusCode < 300
}

func isLoopbackURL(value string) bool {
	return strings.HasPrefix(value, "http://127.0.0.1:") || strings.HasPrefix(value, "http://localhost:") || strings.HasPrefix(value, "http://[::1]:")
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	result := make([]string, 0, len(env)+1)
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			continue
		}
		result = append(result, item)
	}
	return append(result, prefix+value)
}
