package restful

import (
	"database/sql"
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/restful/application"
	"github.com/murkio-dev/vault/internal/restful/audit"
	audit_repositories "github.com/murkio-dev/vault/internal/restful/audit/repositories"
	"github.com/murkio-dev/vault/internal/restful/invitation"
	"github.com/murkio-dev/vault/internal/restful/member"
	member_repositories "github.com/murkio-dev/vault/internal/restful/member/repositories"
	member_usecases "github.com/murkio-dev/vault/internal/restful/member/useCases"
	"github.com/murkio-dev/vault/internal/restful/runtime"
	"github.com/murkio-dev/vault/internal/restful/secret"
	secret_repositories "github.com/murkio-dev/vault/internal/restful/secret/repositories"
	"github.com/murkio-dev/vault/internal/restful/token"
	token_repositories "github.com/murkio-dev/vault/internal/restful/token/repositories"
	"github.com/murkio-dev/vault/pkg/audited"
	"github.com/murkio-dev/vault/pkg/authn"
	"github.com/murkio-dev/vault/pkg/outbox"
	"github.com/murkio-dev/vault/pkg/secretbox"
	"github.com/murkio-dev/vault/pkg/secretvalues"
	"github.com/murkio-dev/vault/pkg/sqlite"
	"github.com/murkio-dev/vault/pkg/tokenauth"
)

func Init(db *sql.DB, box *secretbox.Box, publicURL string, mux *http.ServeMux) (authn.TokenAuthenticator, error) {

	members := member_repositories.NewRepository(db)
	secrets := secret_repositories.NewRepository(db)
	versions := secret_repositories.NewVersionRepository(db)
	tokens := token_repositories.NewRepository(db)
	recorder := audit_repositories.NewRepository(db)

	shared := contracts.Shared{
		Guard:            member_usecases.NewAuthorizeUseCase(members, members, recorder),
		Tx:               audited.New(sqlite.NewTransactor(db), recorder),
		Values:           secretvalues.NewValues(secrets, versions, versions, versions, box),
		AuditCreator:     recorder,
		Outbox:           outbox.New(db),
		MemberFinder:     members,
		MemberUserFinder: members,
		MemberCreator:    members,
		SecretLister:     secrets,
	}

	initializers := []func() error{
		func() error { return application.Init(db, shared, mux) },
		func() error { return member.Init(db, shared, mux) },
		func() error { return invitation.Init(db, shared, mux) },
		func() error { return secret.Init(db, shared, mux) },
		func() error { return token.Init(db, shared, publicURL, mux) },
		func() error { return runtime.Init(shared, mux) },
		func() error { return audit.Init(db, shared, mux) },
	}
	for _, initialize := range initializers {
		if err := initialize(); err != nil {
			return nil, err
		}
	}

	return tokenauth.NewAuthenticator(tokens, tokens), nil
}
