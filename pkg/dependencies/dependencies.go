package dependencies

import (
	"database/sql"

	"github.com/murkio-dev/vault/internal/adapters/audited"
	"github.com/murkio-dev/vault/internal/adapters/secretvalues"
	"github.com/murkio-dev/vault/internal/adapters/tokenauth"
	"github.com/murkio-dev/vault/internal/contracts"
	audit_repositories "github.com/murkio-dev/vault/internal/restful/audit/repositories"
	member_repositories "github.com/murkio-dev/vault/internal/restful/member/repositories"
	member_usecases "github.com/murkio-dev/vault/internal/restful/member/useCases"
	secret_repositories "github.com/murkio-dev/vault/internal/restful/secret/repositories"
	token_repositories "github.com/murkio-dev/vault/internal/restful/token/repositories"
	"github.com/murkio-dev/vault/pkg/authn"
	"github.com/murkio-dev/vault/pkg/secretbox"
	"github.com/murkio-dev/vault/pkg/sqlite"
)

// New monta os contratos que os serviços compartilham (guard, transação
// auditada, cifra e os repositories lidos por mais de um service), sobre o mesmo
// banco, e o autenticador do token do SDK. A API HTTP, o gRPC interno e os testes
// de initializr chamam esta função, cada um com o seu resultado: as peças são só
// repositórios sobre o db e não guardam estado, então os resultados se comportam
// igual. Exemplo: New(db, box) devolve o Dependencies que restful.Init injeta em
// cada service e o autenticador que o middleware e o interceptor gRPC usam.
func New(db *sql.DB, box *secretbox.Box) (contracts.Dependencies, authn.TokenAuthenticator) {
	members := member_repositories.NewRepository(db)
	secrets := secret_repositories.NewRepository(db)
	versions := secret_repositories.NewVersionRepository(db)
	tokens := token_repositories.NewRepository(db)
	recorder := audit_repositories.NewRepository(db)

	deps := contracts.Dependencies{
		Guard:   member_usecases.NewAuthorizeUseCase(members, recorder),
		Tx:      audited.New(sqlite.NewTransactor(db), recorder),
		Values:  secretvalues.NewValues(secrets, versions, box),
		Audit:   recorder,
		Members: members,
		Secrets: secrets,
	}

	return deps, tokenauth.NewAuthenticator(tokens)
}
