package grpcx

import (
	"context"
	"net"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

const (
	grpcRequestIDKey = "x-request-id"
	grpcMaxRequestID = 128
	grpcUserAgent    = "grpc"
)

// Requester monta o ApiHeader de uma chamada gRPC, o equivalente do
// entities.ExtractApiHeader da entrega HTTP, para os grpc_controllers passarem
// o Requester ao use case e a trilha de auditoria correlacionar com o chamador.
// Recebe o ctx do handler, já com o ator posto pelo interceptor de autenticação.
// Devolve o IP do par (loopback no canal interno; vazio se o ctx não tem par), o
// user-agent fixo "grpc", o ator do ctx (zero se não houver) e o request id.
// Exemplo: metadata x-request-id "abc" vira RequestID "abc"; sem metadata, ou
// com um id de mais de 128 bytes, o RequestID é um uuid novo.
// O limite existe porque o id vem do chamador e vai para a trilha: sem ele,
// um cliente gravaria texto de tamanho arbitrário na auditoria.
func Requester(ctx context.Context) entities.ApiHeader {

	ip := ""
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		ip, _, _ = net.SplitHostPort(p.Addr.String())
	}

	requestID := ""
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(grpcRequestIDKey); len(values) > 0 && len(values[0]) <= grpcMaxRequestID {
			requestID = values[0]
		}
	}
	if requestID == "" {
		requestID = uuid.New().String()
	}

	return entities.ApiHeader{ClientIP: ip, UserAgent: grpcUserAgent, RequestID: requestID, Actor: entities.ActorFrom(ctx)}
}
