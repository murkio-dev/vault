package rpc

import (
	"context"
	"net"
	"strings"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/authn"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

const (
	authorizationKey = "authorization"
	requestIDKey     = "x-request-id"
	bearerPrefix     = "bearer "
	maxRequestID     = 128
)

// Authenticate é o interceptor unário da autenticação do canal interno. Lê o
// token do SDK (Bearer vt_...) do metadata "authorization" e o confere no
// banco a cada chamada, como a rota HTTP /api/v1/runtime: revogar o token ou
// remover o membro vale na hora. Só o token do SDK vale; o JWT de pessoa e
// qualquer outro valor recebem Unauthenticated, sempre igual e sem motivo.
// Em caso de sucesso, o ctx do handler leva o ator do token.
func Authenticate(tokens authn.TokenAuthenticator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {

		raw, ok := bearerToken(ctx)
		if !ok || tokens == nil {
			return nil, toStatus(entities.UnauthorizedError())
		}

		actor, err := tokens.Authenticate(ctx, raw)
		if err != nil || actor.Kind != entities.ActorToken {
			return nil, toStatus(entities.UnauthorizedError())
		}

		return handler(entities.WithActor(ctx, actor), req)
	}
}

// requester monta o ApiHeader da chamada: o IP é o do par (loopback), o
// user-agent é fixo e o request id vem do metadata x-request-id (limitado) ou
// é gerado, para a trilha de auditoria correlacionar com o chamador.
func requester(ctx context.Context) entities.ApiHeader {

	ip := ""
	if p, ok := peer.FromContext(ctx); ok {
		ip, _, _ = net.SplitHostPort(p.Addr.String())
	}

	requestID := ""
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(requestIDKey); len(values) > 0 && len(values[0]) <= maxRequestID {
			requestID = values[0]
		}
	}
	if requestID == "" {
		requestID = uuid.New().String()
	}

	return entities.ApiHeader{ClientIP: ip, UserAgent: "grpc", RequestID: requestID, Actor: entities.ActorFrom(ctx)}
}

func bearerToken(ctx context.Context) (string, bool) {

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}

	for _, value := range md.Get(authorizationKey) {
		if len(value) > len(bearerPrefix) && strings.EqualFold(value[:len(bearerPrefix)], bearerPrefix) {
			token := strings.TrimSpace(value[len(bearerPrefix):])
			if strings.HasPrefix(token, authn.TokenPrefix) {
				return token, true
			}
		}
	}

	return "", false
}
