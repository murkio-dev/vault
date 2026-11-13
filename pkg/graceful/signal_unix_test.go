//go:build unix

package graceful_test

import (
	"context"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/murkio-dev/vault/pkg/graceful"
)

// Main test

func TestRunStopsOnSIGTERM(t *testing.T) {
	t.Run("Quando chega SIGTERM.", func(t *testing.T) { whenSIGTERMArrives(t) })
}

// Functions

func whenSIGTERMArrives(t *testing.T) {
	server := newFake()
	done := make(chan error, 1)
	go func() { done <- graceful.Run(context.Background(), time.Second, server) }()
	<-server.started
	time.Sleep(50 * time.Millisecond)

	assert.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))

	select {
	case err := <-done:
		assert.NoError(t, err, "SIGTERM é desligamento normal, não erro")
	case <-time.After(5 * time.Second):
		t.Fatal("SIGTERM não parou o servidor")
	}
	assert.EqualValues(t, 1, server.shutdowns.Load())
}
