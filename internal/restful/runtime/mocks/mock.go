package mocks

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	audit_mocks "github.com/murkio-dev/vault/internal/restful/audit/mocks"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
	secret_mocks "github.com/murkio-dev/vault/internal/restful/secret/mocks"
)

// Mocks

// EnvRig reúne os fakes que o use case de leitura do ambiente recebe. O runtime
// não tem repository próprio: lê os segredos do membro, decifra, audita e passa
// pelo guard, então o rig só junta os fakes dos outros services. Os campos são
// os próprios fakes: o teste troca uma *Func ou um erro neles e depois confere
// o que foi gravado. Member é o membro dono do ambiente lido (o uri_id).
// Exemplo: r := NewEnvRig(); r.WithSecrets("A", "B") faz a listagem devolver
// dois segredos com esses nomes.
type EnvRig struct {
	Secrets *secret_mocks.SecretRepo
	Values  *secret_mocks.Values
	Guard   *member_mocks.Guard
	Tx      *audit_mocks.Tx
	Audit   *audit_mocks.AuditRepo
	Member  entities.Member
}

// Helpers

func NewEnvRig() *EnvRig {
	member := member_mocks.NewMember(entities.RoleDeveloper)

	return &EnvRig{
		Secrets: &secret_mocks.SecretRepo{},
		Values:  &secret_mocks.Values{ValueResult: "valor-secreto"},
		Guard:   &member_mocks.Guard{Member: member},
		Tx:      &audit_mocks.Tx{},
		Audit:   &audit_mocks.AuditRepo{},
		Member:  member,
	}
}

func (r *EnvRig) WithSecrets(names ...string) {
	r.Secrets.ListFunc = func(context.Context, uuid.UUID, uuid.UUID, int, int) ([]entities.Secret, int, error) {
		list := make([]entities.Secret, len(names))
		for i, name := range names {
			list[i] = secret_mocks.NewSecret()
			list[i].Name = name
		}
		return list, len(list), nil
	}
}
