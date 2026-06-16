package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeMonitor struct {
	starts atomic.Int32
	stops  atomic.Int32
}

func (m *fakeMonitor) Start(ctx context.Context) {
	m.starts.Add(1)
}

func (m *fakeMonitor) Stop() {
	m.stops.Add(1)
}

func TestServerSocketPermissions(t *testing.T) {
	socketPath := filepath.Join(serverTestDir(t), "watchcow.sock")
	srv := New(socketPath, http.NewServeMux(), nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	select {
	case <-srv.Ready():
	case err := <-errCh:
		if isSandboxListenPermissionError(err) {
			t.Skipf("socket listen not permitted in this sandbox: %v", err)
		}
		t.Fatalf("server failed before ready: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("server did not become ready")
	}

	info, err := os.Stat(socketPath)
	if err != nil {
		t.Fatalf("socket stat failed: %v", err)
	}
	if got := info.Mode().Perm(); got != socketFileMode {
		t.Errorf("socket mode = %o, want %o", got, socketFileMode)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start() returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop after context cancellation")
	}
}

func TestServerStopIsIdempotent(t *testing.T) {
	socketPath := filepath.Join(serverTestDir(t), "watchcow.sock")
	monitor := &fakeMonitor{}
	srv := New(socketPath, http.NewServeMux(), monitor)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	select {
	case <-srv.Ready():
	case err := <-errCh:
		if isSandboxListenPermissionError(err) {
			t.Skipf("socket listen not permitted in this sandbox: %v", err)
		}
		t.Fatalf("server failed before ready: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("server did not become ready")
	}

	if err := srv.Stop(context.Background()); err != nil {
		t.Fatalf("first Stop() error = %v", err)
	}
	if err := srv.Stop(context.Background()); err != nil {
		t.Fatalf("second Stop() error = %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start() returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop")
	}

	if got := monitor.stops.Load(); got != 1 {
		t.Errorf("monitor Stop calls = %d, want 1", got)
	}
}

func isSandboxListenPermissionError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "operation not permitted")
}

func serverTestDir(t *testing.T) string {
	t.Helper()

	if info, err := os.Stat("/private/tmp"); err == nil && info.IsDir() {
		dir, err := os.MkdirTemp("/private/tmp", "watchcow-server-*")
		if err == nil {
			t.Cleanup(func() { os.RemoveAll(dir) })
			return dir
		}
	}

	return t.TempDir()
}
