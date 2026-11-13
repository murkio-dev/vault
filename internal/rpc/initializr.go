package rpc

import (
	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/restful/runtime/useCases"
	"github.com/murkio-dev/vault/pkg/authn"
)

func Init(shared contracts.Shared, tokens authn.TokenAuthenticator, port string) (*Server, error) {

	env := useCases.NewReadEnvUseCase(shared.SecretLister, shared.Values, shared.Guard, shared.Tx, shared.AuditCreator)

	return NewServer(port, tokens, NewVaultController(env))
}
