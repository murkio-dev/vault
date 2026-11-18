package grpcx

import (
	"context"
	"strings"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/authn"
	vaultv1 "github.com/murkio-dev/vault/pkg/rpc/vaultv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	authorizationKey = "authorization"
	bearerPrefix     = "bearer "
)

// runtimeMethodPrefix marca os métodos do VaultService, o único serviço que
// aceita o token do SDK. O nome vem do código gerado, para acompanhar o .proto.
var runtimeMethodPrefix = "/" + vaultv1.VaultService_ServiceDesc.ServiceName + "/"

// Authenticate é o interceptor unário da autenticação do canal interno. Lê a
// credencial do metadata "authorization" (Bearer ...) e aplica a mesma regra da
// API HTTP (authn.Identify): os métodos do VaultService (o runtime) só aceitam
// o token do SDK (sk_vt_...), conferido no banco a cada chamada; todos os outros
// serviços só aceitam o JWT de pessoa emitido pelo Auth.
// Recebe o verificador do JWT e o autenticador de tokens; com um deles nil, as
// chamadas que dependem dele são recusadas.
// Credencial ausente, do tipo errado para o método ou inválida recebe
// Unauthenticated, sempre igual e sem motivo. Em caso de sucesso, o ctx do
// handler leva o ator.
// Exemplo: um JWT em /murkio.vault.v1.VaultService/ReadEnv é recusado, e um
// sk_vt_... em /murkio.vault.v1.ApplicationService/ListApplications também.
// Um serviço novo cai na regra do JWT por padrão: o token do SDK só passa a
// valer em outro serviço se alguém mudar este arquivo de propósito.
func Authenticate(verifier *authn.Verifier, tokens authn.TokenAuthenticator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {

		sdkOnly := info != nil && strings.HasPrefix(info.FullMethod, runtimeMethodPrefix)

		actor, ok := authn.Identify(ctx, bearerToken(ctx), sdkOnly, verifier, tokens)
		if !ok {
			return nil, Error(entities.UnauthorizedError())
		}

		return handler(entities.WithActor(ctx, actor), req)
	}
}

func bearerToken(ctx context.Context) string {

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	values := md.Get(authorizationKey)
	if len(values) != 1 {
		return ""
	}

	value := values[0]
	if len(value) <= len(bearerPrefix) || !strings.EqualFold(value[:len(bearerPrefix)], bearerPrefix) {
		return ""
	}

	return strings.TrimSpace(value[len(bearerPrefix):])
}
