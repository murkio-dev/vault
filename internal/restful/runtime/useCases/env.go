package useCases

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
)

const MaxNames = 100

// Builder

type ReadEnvUseCase struct {
	Secrets contracts.ISecretLister
	Values  contracts.ISecretValues
	Guard   contracts.IAccessGuard
	Tx      contracts.IAuditedTx
	Audit   contracts.IAuditLogCreator
}

func NewReadEnvUseCase(secrets contracts.ISecretLister, values contracts.ISecretValues, guard contracts.IAccessGuard, tx contracts.IAuditedTx, audit contracts.IAuditLogCreator) ReadEnvUseCase {
	return ReadEnvUseCase{Secrets: secrets, Values: values, Guard: guard, Tx: tx, Audit: audit}
}

// DTOS

type ReadEnvInput struct {
	Requester entities.ApiHeader
	UriID     string
	Names     []string
}

type Environment struct {
	UriID     string                     `json:"uri_id"`
	Variables []entities.RuntimeVariable `json:"variables"`
}

type ReadEnvOutput struct {
	Data  Environment     `json:"data"`
	Error *entities.Error `json:"error,omitempty"`
}

// UseCase

// Perform devolve as variáveis do membro dono do token, com as referências
// resolvidas. O token identifica o membro; o uri_id do caminho só confirma que
// o SDK está falando com a tabela certa, e outro uri_id responde 404 (e fica na
// trilha). A leitura é auditada na mesma transação: sem trilha, nada sai. Uma
// referência cujo alvo sumiu é omitida em vez de derrubar o ambiente inteiro.
func (u ReadEnvUseCase) Perform(ctx context.Context, input ReadEnvInput) ReadEnvOutput {

	actor := input.Requester.Actor
	if actor.Kind != entities.ActorToken {
		unauthorized := entities.UnauthorizedError()
		return ReadEnvOutput{Error: &unauthorized}
	}

	if len(input.Names) > MaxNames {
		invalid := entities.ValidationError("Too many names.", entities.InvalidParam{Field: "names", Reason: "at most 100"})
		return ReadEnvOutput{Error: &invalid}
	}

	guard := u.Guard.Perform(ctx, entities.AuthorizeInput{Requester: input.Requester, Application: actor.ApplicationID, Action: entities.ActionEnvRead, Owner: actor.MemberID})
	member, denied := guard.Member, guard.Error
	if denied != nil {
		return ReadEnvOutput{Error: denied}
	}

	if subtle.ConstantTimeCompare([]byte(member.UriID), []byte(input.UriID)) != 1 {
		u.deny(ctx, input.Requester, actor.ApplicationID)
		notFound := entities.NotFoundError("Resource")
		return ReadEnvOutput{Error: &notFound}
	}

	secrets, _, err := u.Secrets.List(ctx, member.ApplicationID, member.ID, entities.MaxSecretsPerTable, 0)
	if err != nil {
		internal := entities.InternalServerError(err, nil)
		return ReadEnvOutput{Error: &internal}
	}

	wanted := toSet(input.Names)
	variables := make([]entities.RuntimeVariable, 0, len(secrets))

	entry := entities.NewAuditLog(input.Requester, member.ApplicationID, entities.AuditEnvRead, entities.AuditAllowed,
		entities.AuditTarget{Type: "member", ID: member.ID.String()}, map[string]any{"member_id": member.ID.String()})

	err = u.Tx.Run(ctx, entry, func(ctx context.Context) error {
		variables = variables[:0]
		for _, secret := range secrets {
			if len(wanted) > 0 && !wanted[secret.Name] {
				continue
			}

			value, version, err := u.Values.Value(ctx, secret, 0)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			variables = append(variables, entities.RuntimeVariable{Name: secret.Name, Type: secret.Type, Version: version, Value: value})
		}
		return nil
	})
	if err != nil {
		internal := entities.InternalServerError(err, nil)
		return ReadEnvOutput{Error: &internal}
	}

	return ReadEnvOutput{Data: Environment{UriID: member.UriID, Variables: variables}}
}

func (u ReadEnvUseCase) deny(ctx context.Context, requester entities.ApiHeader, application uuid.UUID) {
	_, _ = u.Audit.Create(ctx, entities.NewAuditLog(requester, application, entities.AuditAccessDenied, entities.AuditDenied,
		entities.AuditTarget{}, map[string]any{"action": string(entities.ActionEnvRead), "reason": "uri mismatch"}))
}

func toSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}
