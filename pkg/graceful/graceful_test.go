package graceful_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/murkio-dev/vault/pkg/graceful"
)

// Main test

func TestRun(t *testing.T) {
	t.Run("Quando o contexto é cancelado.", func(t *testing.T) { whenContextIsCanceled(t) })
	t.Run("Quando uma requisição está em andamento.", func(t *testing.T) { whenRequestIsInFlight(t) })
	t.Run("Quando um servidor cai sozinho.", func(t *testing.T) { whenServerFails(t) })
	t.Run("Quando o desligamento passa do prazo.", func(t *testing.T) { whenShutdownTimesOut(t) })
}

// Mocks

type fakeServer struct {
	started     chan struct{}
	stopped     chan struct{}
	serveErr    error
	shutdownErr error
	shutdowns   atomic.Int32
	hang        bool
}

func newFake() *fakeServer {
	return &fakeServer{started: make(chan struct{}), stopped: make(chan struct{})}
}

func (f *fakeServer) Serve() error {
	close(f.started)
	if f.serveErr != nil {
		return f.serveErr
	}
	<-f.stopped
	return nil
}

func (f *fakeServer) Shutdown(ctx context.Context) error {
	f.shutdowns.Add(1)
	if f.hang {
		<-ctx.Done()
	}
	select {
	case <-f.stopped:
	default:
		close(f.stopped)
	}
	if f.hang {
		return ctx.Err()
	}
	return f.shutdownErr
}

// Helpers

func freeAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())
	return addr
}

// Functions

func whenContextIsCanceled(t *testing.T) {
	first, second := newFake(), newFake()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- graceful.Run(ctx, time.Second, first, second) }()
	<-first.started
	<-second.started

	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err, "desligamento limpo devolve nil")
	case <-time.After(5 * time.Second):
		t.Fatal("Run não voltou")
	}
	assert.EqualValues(t, 1, first.shutdowns.Load())
	assert.EqualValues(t, 1, second.shutdowns.Load())
}

func whenRequestIsInFlight(t *testing.T) {
	addr := freeAddr(t)
	entered, release := make(chan struct{}), make(chan struct{})
	srv := &http.Server{Addr: addr, Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		_, _ = io.WriteString(w, "terminou")
	})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- graceful.Run(ctx, 5*time.Second, graceful.HTTP(srv)) }()

	var response *http.Response
	var body []byte
	requestDone := make(chan error, 1)
	go func() {
		var err error
		for range 50 {
			response, err = http.Get("http://" + addr)
			if err == nil {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if err == nil {
			body, err = io.ReadAll(response.Body)
			_ = response.Body.Close()
		}
		requestDone <- err
	}()
	<-entered

	cancel()
	select {
	case <-done:
		t.Fatal("Run voltou com uma requisição ainda em andamento")
	case <-time.After(200 * time.Millisecond):
	}
	close(release)

	require.NoError(t, <-requestDone)
	assert.Equal(t, "terminou", string(body), "a requisição em andamento termina em vez de ser cortada")
	require.NoError(t, <-done)
}

func whenServerFails(t *testing.T) {
	failing, healthy := newFake(), newFake()
	failing.serveErr = errors.New("porta em uso")

	err := graceful.Run(context.Background(), time.Second, failing, healthy)

	assert.ErrorContains(t, err, "porta em uso")
	assert.EqualValues(t, 1, healthy.shutdowns.Load(), "quando um cai, o outro é parado")
}

func whenShutdownTimesOut(t *testing.T) {
	stuck := newFake()
	stuck.hang = true
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- graceful.Run(ctx, 100*time.Millisecond, stuck) }()
	<-stuck.started

	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(5 * time.Second):
		t.Fatal("o prazo do desligamento não foi respeitado")
	}
}
