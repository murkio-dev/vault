package grpcx_test

import (
	"context"
	"net"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/grpcx"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// Main test

func TestRequester(t *testing.T) {
	t.Run("Quando o ctx traz par, ator e request id.", func(t *testing.T) { whenGRPCRequesterIsComplete(t) })
	t.Run("Quando o ctx vem vazio.", func(t *testing.T) { whenGRPCRequesterIsEmpty(t) })
	t.Run("Quando o request id é grande demais.", func(t *testing.T) { whenGRPCRequestIDIsTooLong(t) })
}

// Functions

func whenGRPCRequesterIsComplete(t *testing.T) {
	actor := entities.Actor{Kind: entities.ActorToken, UserID: uuid.NewV7(), TokenID: uuid.NewV7()}
	ctx := entities.WithActor(grpcContext("req-123"), actor)
	ctx = peer.NewContext(ctx, &peer.Peer{Addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 50000}})

	requester := grpcx.Requester(ctx)

	assert.Equal(t, "127.0.0.1", requester.ClientIP)
	assert.Equal(t, "grpc", requester.UserAgent)
	assert.Equal(t, "req-123", requester.RequestID)
	assert.Equal(t, actor, requester.Actor)
}

func whenGRPCRequesterIsEmpty(t *testing.T) {
	first := grpcx.Requester(context.Background())
	second := grpcx.Requester(context.Background())

	assert.Empty(t, first.ClientIP)
	assert.Equal(t, entities.Actor{}, first.Actor, "sem interceptor, o ator é o zero e o use case recusa")
	_, err := uuid.Parse(first.RequestID)
	assert.NoError(t, err, "sem x-request-id, o id é gerado")
	assert.NotEqual(t, first.RequestID, second.RequestID)
}

func whenGRPCRequestIDIsTooLong(t *testing.T) {
	limit := strings.Repeat("a", 128)
	tooLong := strings.Repeat("a", 129)

	accepted := grpcx.Requester(grpcContext(limit))
	replaced := grpcx.Requester(grpcContext(tooLong))

	assert.Equal(t, limit, accepted.RequestID)
	assert.NotEqual(t, tooLong, replaced.RequestID, "o cliente não grava texto de tamanho arbitrário na trilha")
	_, err := uuid.Parse(replaced.RequestID)
	assert.NoError(t, err)
}

func grpcContext(requestID string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-request-id", requestID))
}
