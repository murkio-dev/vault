package httpx_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/murkio-dev/vault/pkg/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestServer(t *testing.T) {
	t.Run("Quando o servidor atende e para com o Shutdown.", func(t *testing.T) { whenServerServesAndShutsDown(t) })
	t.Run("Quando o handler dá panic dentro do servidor.", func(t *testing.T) { whenServerRecoversFromPanic(t) })
	t.Run("Quando o corpo passa do teto.", func(t *testing.T) { whenBodyIsOverTheLimit(t) })
	t.Run("Quando a porta já está em uso.", func(t *testing.T) { whenServerPortIsInUse(t) })
	t.Run("Quando a porta ou os parâmetros são inválidos.", func(t *testing.T) { whenServerInputIsInvalid(t) })
	t.Run("Quando o Shutdown estoura o prazo.", func(t *testing.T) { whenShutdownTimesOut(t) })
}

// Functions

func startServer(t *testing.T, handler http.Handler) (*httpx.Server, string, chan error) {
	t.Helper()
	logs, path := newLogger(t)

	server, err := httpx.NewServer(httpx.ServerInput{Service: "vault", Port: "0", Logs: logs, Handler: handler})
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() { done <- server.Serve() }()

	return server, path, done
}

func urlOf(t *testing.T, server *httpx.Server) string {
	t.Helper()
	_, port, err := net.SplitHostPort(server.Addr())
	require.NoError(t, err)
	return "http://127.0.0.1:" + port
}

func stop(t *testing.T, server *httpx.Server, done chan error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	assert.NoError(t, server.Shutdown(ctx))
	assert.NoError(t, <-done, "o Shutdown não é falha do Serve")
}

func whenServerServesAndShutsDown(t *testing.T) {
	server, path, done := startServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	response, err := http.Get(urlOf(t, server))
	require.NoError(t, err)
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "ok", string(body))
	stop(t, server, done)

	log, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(log), "http listening on", "o boot sai no log sem o main fazer nada")
	assert.Contains(t, string(log), `"service.name":"vault"`)
}

func whenServerRecoversFromPanic(t *testing.T) {
	server, path, done := startServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("segredo-que-nao-pode-vazar")
	}))

	response, err := http.Get(urlOf(t, server))
	require.NoError(t, err, "o panic não derruba a conexão")
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
	assert.NotContains(t, string(body), "segredo-que-nao-pode-vazar")
	stop(t, server, done)

	log, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(log), "panic recovered")
}

func whenBodyIsOverTheLimit(t *testing.T) {
	server, _, done := startServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			var tooLarge *http.MaxBytesError
			assert.True(t, errors.As(err, &tooLarge))
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	atLimit, err := http.Post(urlOf(t, server), "application/json", strings.NewReader(strings.Repeat("a", httpx.MaxBodyBytes)))
	require.NoError(t, err)
	atLimit.Body.Close()
	over, err := http.Post(urlOf(t, server), "application/json", strings.NewReader(strings.Repeat("a", httpx.MaxBodyBytes+1)))
	require.NoError(t, err)
	over.Body.Close()

	assert.Equal(t, http.StatusNoContent, atLimit.StatusCode, "no teto ainda passa")
	assert.Equal(t, http.StatusRequestEntityTooLarge, over.StatusCode, "um byte a mais é recusado")
	stop(t, server, done)
}

func whenServerPortIsInUse(t *testing.T) {
	first, _, done := startServer(t, http.NotFoundHandler())
	_, port, err := net.SplitHostPort(first.Addr())
	require.NoError(t, err)
	logs, _ := newLogger(t)

	second, err := httpx.NewServer(httpx.ServerInput{Service: "vault", Port: port, Logs: logs, Handler: http.NotFoundHandler()})

	assert.Error(t, err)
	assert.Nil(t, second)
	stop(t, first, done)
}

func whenServerInputIsInvalid(t *testing.T) {
	logs, _ := newLogger(t)

	_, badPort := httpx.NewServer(httpx.ServerInput{Service: "vault", Port: "nao-e-porta", Logs: logs, Handler: http.NotFoundHandler()})
	_, noLogs := httpx.NewServer(httpx.ServerInput{Service: "vault", Port: "0", Handler: http.NotFoundHandler()})
	_, noHandler := httpx.NewServer(httpx.ServerInput{Service: "vault", Port: "0", Logs: logs})

	assert.Error(t, badPort)
	if assert.Error(t, noLogs) {
		assert.Contains(t, noLogs.Error(), "input.Logs")
	}
	if assert.Error(t, noHandler) {
		assert.Contains(t, noHandler.Error(), "input.Handler")
	}
}

func whenShutdownTimesOut(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	server, _, done := startServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))

	go func() {
		if response, err := http.Get(urlOf(t, server)); err == nil {
			response.Body.Close()
		}
	}()
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	assert.ErrorIs(t, server.Shutdown(ctx), context.DeadlineExceeded, "com uma requisição presa, o prazo estoura e o erro sobe")
	assert.NoError(t, <-done, "mesmo assim o Serve termina sem erro")
}
