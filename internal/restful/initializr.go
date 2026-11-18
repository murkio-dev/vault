package restful

import (
	"database/sql"
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/restful/application"
	"github.com/murkio-dev/vault/internal/restful/audit"
	"github.com/murkio-dev/vault/internal/restful/invitation"
	"github.com/murkio-dev/vault/internal/restful/member"
	"github.com/murkio-dev/vault/internal/restful/runtime"
	"github.com/murkio-dev/vault/internal/restful/secret"
	"github.com/murkio-dev/vault/internal/restful/token"
	"github.com/murkio-dev/vault/pkg/authn"
	"github.com/murkio-dev/vault/pkg/dependencies"
	"github.com/murkio-dev/vault/pkg/grpcx"
	"github.com/murkio-dev/vault/pkg/secretbox"
)

func Init(db *sql.DB, box *secretbox.Box, publicURL string, mux *http.ServeMux) (authn.TokenAuthenticator, error) {

	deps, tokenAuthenticator := dependencies.New(db, box)

	initializers := []func() error{
		func() error { return application.Init(db, deps, mux) },
		func() error { return member.Init(db, deps, mux) },
		func() error { return invitation.Init(db, deps, mux) },
		func() error { return secret.Init(db, deps, mux) },
		func() error { return token.Init(db, deps, publicURL, mux) },
		func() error { return runtime.Init(deps, mux) },
		func() error { return audit.Init(db, deps, mux) },
	}
	for _, initialize := range initializers {
		if err := initialize(); err != nil {
			return nil, err
		}
	}

	return tokenAuthenticator, nil
}

// InitGRPC sobe o gRPC interno: abre o listener em 127.0.0.1:port com o
// interceptor de autenticação e registra os handlers de cada service. Se o
// registro de algum falhar, o listener é fechado e nada sobe.
func InitGRPC(db *sql.DB, deps contracts.Dependencies, verifier *authn.Verifier, tokens authn.TokenAuthenticator, publicURL, port string) (*grpcx.Server, error) {

	server, err := grpcx.NewServer(port, verifier, tokens)
	if err != nil {
		return nil, err
	}

	registrar := server.Registrar()
	initializers := []func() error{
		func() error { return runtime.InitGRPC(deps, registrar) },
		func() error { return application.InitGRPC(db, deps, registrar) },
		func() error { return member.InitGRPC(db, deps, registrar) },
		func() error { return invitation.InitGRPC(db, deps, registrar) },
		func() error { return secret.InitGRPC(db, deps, registrar) },
		func() error { return token.InitGRPC(db, deps, publicURL, registrar) },
		func() error { return audit.InitGRPC(db, deps, registrar) },
	}
	for _, initialize := range initializers {
		if err := initialize(); err != nil {
			_ = server.Close()
			return nil, err
		}
	}

	return server, nil
}
