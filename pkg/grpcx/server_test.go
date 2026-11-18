package grpcx_test

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/pkg/authn/authntest"
	"github.com/murkio-dev/vault/pkg/grpcx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const blockMethod = "/test.Block/Wait"

// Main test

func TestServer(t *testing.T) {
	t.Run("Quando abre o listener só em loopback.", func(t *testing.T) { whenServerListensOnLoopback(t) })
	t.Run("Quando a porta já está em uso.", func(t *testing.T) { whenPortIsInUse(t) })
	t.Run("Quando a porta é inválida.", func(t *testing.T) { whenPortIsInvalid(t) })
	t.Run("Quando o servidor para com GracefulStop.", func(t *testing.T) { whenServerGracefullyStops(t) })
	t.Run("Quando desliga sem chamadas em andamento.", func(t *testing.T) { whenShutdownIsIdle(t) })
	t.Run("Quando o prazo do desligamento vence com uma chamada em andamento.", func(t *testing.T) { whenShutdownTimesOut(t) })
	t.Run("Quando descarta um servidor que não subiu.", func(t *testing.T) { whenServerIsClosedBeforeServing(t) })
}

// Functions

func newServer(t *testing.T) *grpcx.Server {
	t.Helper()

	server, err := grpcx.NewServer("0", authntest.New(t).Verifier(), nil)
	require.NoError(t, err)

	return server
}

// serve sobe o Serve e só volta quando o servidor já aceita conexões, para o
// teste não parar o servidor antes de ele começar (o Serve devolveria
// grpc.ErrServerStopped).
func serve(t *testing.T, server *grpcx.Server) chan error {
	t.Helper()

	done := make(chan error, 1)
	go func() { done <- server.Serve() }()

	conn, err := grpc.NewClient(server.Addr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn.Connect()
	for state := conn.GetState(); state != connectivity.Ready; state = conn.GetState() {
		require.True(t, conn.WaitForStateChange(ctx, state), "o servidor não ficou pronto a tempo")
	}

	return done
}

func whenServerListensOnLoopback(t *testing.T) {
	server := newServer(t)
	defer server.Close()

	host, port, err := net.SplitHostPort(server.Addr())

	assert.NoError(t, err)
	assert.Equal(t, "127.0.0.1", host, "o host não é configurável: o canal nunca sai da máquina")
	assert.NotEqual(t, "0", port, "a porta 0 escolhe uma livre")
	assert.NotNil(t, server.Registrar())
}

func whenPortIsInUse(t *testing.T) {
	first := newServer(t)
	defer first.Close()
	_, port, _ := net.SplitHostPort(first.Addr())

	second, err := grpcx.NewServer(port, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, second)
	assert.True(t, strings.HasPrefix(err.Error(), "grpc: "))
}

func whenPortIsInvalid(t *testing.T) {
	server, err := grpcx.NewServer("nao-e-porta", nil, nil)

	assert.Error(t, err)
	assert.Nil(t, server)
}

func whenServerGracefullyStops(t *testing.T) {
	server := newServer(t)
	done := serve(t, server)

	server.GracefulStop()

	assert.NoError(t, <-done)
}

func whenShutdownIsIdle(t *testing.T) {
	server := newServer(t)
	done := serve(t, server)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	assert.NoError(t, server.Shutdown(ctx))
	assert.NoError(t, <-done)
}

func whenShutdownTimesOut(t *testing.T) {
	auth := authntest.New(t)
	server, err := grpcx.NewServer("0", auth.Verifier(), nil)
	require.NoError(t, err)

	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	server.Registrar().RegisterService(&grpc.ServiceDesc{
		ServiceName: "test.Block",
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{{
			MethodName: "Wait",
			Handler: func(_ any, ctx context.Context, _ func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
				close(started)
				select {
				case <-release:
				case <-ctx.Done():
				}
				return wrapperspb.String("fim"), nil
			},
		}},
	}, struct{}{})
	done := serve(t, server)

	conn, err := grpc.NewClient(server.Addr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	token := auth.Token(t, uuid.NewV7(), nil, time.Minute)
	invoked := make(chan error, 1)
	go func() {
		ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
		invoked <- conn.Invoke(ctx, blockMethod, wrapperspb.String("oi"), &wrapperspb.StringValue{})
	}()
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	assert.ErrorIs(t, server.Shutdown(ctx), context.DeadlineExceeded, "a chamada em andamento é cortada, e o prazo nunca é passado")
	assert.Error(t, <-invoked, "o cliente da chamada cortada recebe erro")
	assert.NoError(t, <-done)
}

func whenServerIsClosedBeforeServing(t *testing.T) {
	server := newServer(t)
	addr := server.Addr()

	assert.NoError(t, server.Close())

	listener, err := net.Listen("tcp", addr)
	if assert.NoError(t, err, "a porta foi liberada") {
		listener.Close()
	}
}
