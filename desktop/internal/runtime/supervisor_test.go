package runtime

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/1085924051/modelctl/desktop/internal/api"
)

func TestSupervisorHelperProcess(t *testing.T) {
	if os.Getenv("MODELCTL_SUPERVISOR_HELPER") != "1" {
		return
	}
	port, err := strconv.Atoi(os.Getenv("MODELCTL_SUPERVISOR_PORT"))
	if err != nil {
		os.Exit(2)
	}
	server := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", port), Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.Header().Set("content-type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		http.NotFound(w, r)
	})}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		os.Exit(3)
	}
	os.Exit(0)
}

func TestSupervisorStartsAndStopsOwnedProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper is covered by the Windows package smoke test")
	}
	port := freePort(t)
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	node := writeHelperScript(t)
	paths := Paths{
		Root:             t.TempDir(),
		NodeBinary:       node,
		ControlPlaneRoot: t.TempDir(),
		PythonBinary:     filepath.Join(t.TempDir(), "python"),
		LogDir:           t.TempDir(),
	}
	t.Setenv("MODELCTL_URL", url)
	t.Setenv("MODELCTL_SUPERVISOR_HELPER", "1")
	t.Setenv("MODELCTL_SUPERVISOR_PORT", strconv.Itoa(port))
	supervisor := NewSupervisor(paths, t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	baseURL, err := supervisor.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if baseURL != url {
		t.Fatalf("base URL = %q, want %q", baseURL, url)
	}
	if err := api.NewClient(baseURL).Health(); err != nil {
		t.Fatal(err)
	}
	if err := supervisor.Close(); err != nil {
		t.Fatal(err)
	}
	if err := api.NewClient(baseURL).Health(); err == nil {
		t.Fatal("owned daemon remained healthy after Close")
	}
}

func TestSupervisorDoesNotStartExternalHealthyService(t *testing.T) {
	port := freePort(t)
	server := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", port), Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"status":"ok"}`)) })}
	go func() { _ = server.ListenAndServe() }()
	defer server.Close()
	waitForHealth(t, fmt.Sprintf("http://127.0.0.1:%d", port))
	paths := Paths{NodeBinary: filepath.Join(t.TempDir(), "missing-node")}
	t.Setenv("MODELCTL_URL", fmt.Sprintf("http://127.0.0.1:%d", port))
	supervisor := NewSupervisor(paths, t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := supervisor.Start(ctx); err != nil {
		t.Fatalf("Start() rejected healthy external service: %v", err)
	}
	if err := supervisor.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSupervisorTimesOutWhenChildNeverBecomesReady(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper is covered by the Windows package smoke test")
	}
	paths := Paths{NodeBinary: os.Args[0], ControlPlaneRoot: t.TempDir(), LogDir: t.TempDir()}
	port := freePort(t)
	t.Setenv("MODELCTL_URL", fmt.Sprintf("http://127.0.0.1:%d", port))
	t.Setenv("MODELCTL_SUPERVISOR_HELPER", "0")
	supervisor := NewSupervisor(paths, t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()
	if _, err := supervisor.Start(ctx); err == nil {
		t.Fatal("Start() succeeded without a ready child")
	}
	_ = supervisor.Close()
}

func writeHelperScript(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-node.sh")
	body := "#!/bin/sh\nexec \"" + os.Args[0] + "\" -test.run=TestSupervisorHelperProcess\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func waitForHealth(t *testing.T, baseURL string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(baseURL + "/health")
		if err == nil {
			response.Body.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("service at %s did not become ready", baseURL)
}
