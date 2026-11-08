package tokenauth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/authn"
)

// maxRawToken limita o que é nem procurado: um token de verdade tem 46
// caracteres.
const maxRawToken = 128

var errInvalidToken = errors.New("invalid token")

// Authenticator é o authn.TokenAuthenticator: transforma o token do SDK no ator
// que ele representa.
type Authenticator struct {
	Finder  contracts.ITokenHashFinder
	Toucher contracts.ITokenUsageToucher
}

func NewAuthenticator(finder contracts.ITokenHashFinder, toucher contracts.ITokenUsageToucher) Authenticator {
	return Authenticator{Finder: finder, Toucher: toucher}
}

var _ authn.TokenAuthenticator = Authenticator{}

// Authenticate busca o token pelo hash a cada requisição, no banco: revogar,
// expirar, suspender o membro ou apagar a aplicação valem na hora. Formato
// errado (sem o prefixo vt_ ou grande demais) nem chega ao banco. Todo motivo
// de recusa vira o mesmo erro, sem detalhe, e a falha do banco também nega.
// O último uso é só informação: se gravá-lo falhar, o token continua valendo.
// Exemplo: um token revogado um segundo atrás já recebe erro.
func (a Authenticator) Authenticate(ctx context.Context, raw string) (entities.Actor, error) {

	if !strings.HasPrefix(raw, entities.SDKTokenPrefix) || len(raw) > maxRawToken {
		return entities.Actor{}, errInvalidToken
	}

	token, err := a.Finder.FindActiveByHash(ctx, entities.HashToken(raw))
	if errors.Is(err, sql.ErrNoRows) {
		return entities.Actor{}, errInvalidToken
	}
	if err != nil {
		return entities.Actor{}, err
	}

	now := time.Now().UTC()
	_, _ = a.Toucher.TouchLastUsed(ctx, entities.AccessToken{ID: token.ID, ApplicationID: token.ApplicationID, LastUsedAt: &now})

	return entities.Actor{
		Kind:          entities.ActorToken,
		UserID:        token.UserID,
		TokenID:       token.ID,
		MemberID:      token.MemberID,
		ApplicationID: token.ApplicationID,
	}, nil
}
