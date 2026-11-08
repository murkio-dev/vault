package useCases_test

import (
	"context"
	audit_mocks "github.com/murkio-dev/vault/internal/restful/audit/mocks"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/murkio-dev/vault/internal/restful/secret/mocks"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
)

// Helpers

// rig reúne os fakes que todo use case de segredo recebe. O guard libera
// tudo e resolve o dono para Owner (um membro novo).
type rig struct {
	secrets  *mocks.SecretRepo
	versions *mocks.VersionRepo
	values   *mocks.Values
	guard    *member_mocks.Guard
	tx       *audit_mocks.Tx
	owner    uuid.UUID
}

func newRig() *rig {
	r := &rig{
		secrets: mocks.NewRepo(), versions: &mocks.VersionRepo{}, values: &mocks.Values{ValueResult: "valor-revelado"},
		guard: &member_mocks.Guard{}, tx: &audit_mocks.Tx{}, owner: uuid.NewV7(),
	}
	r.guard.PerformFunc = func(_ context.Context, _ entities.AuthorizeInput) entities.AuthorizeOutput {
		return entities.AuthorizeOutput{Owner: r.owner}
	}
	return r
}

func (r *rig) requester() entities.ApiHeader { return member_mocks.NewUserRequester() }
