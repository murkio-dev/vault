package useCases

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
)

// MaxCreateBatch limita quantos segredos uma chamada cria.
const MaxCreateBatch = 50

// Builder

type CreateSecretUseCase struct {
	Counter contracts.ISecretCounter
	Creator contracts.ISecretCreator
	Finder  contracts.ISecretFinder
	Values  contracts.ISecretValues
	Guard   contracts.IAccessGuard
	Tx      contracts.IAuditedTx
}

func NewCreateSecretUseCase(counter contracts.ISecretCounter, creator contracts.ISecretCreator, finder contracts.ISecretFinder, values contracts.ISecretValues, guard contracts.IAccessGuard, tx contracts.IAuditedTx) CreateSecretUseCase {
	return CreateSecretUseCase{Counter: counter, Creator: creator, Finder: finder, Values: values, Guard: guard, Tx: tx}
}

// DTOS

type CreateInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Member      uuid.UUID
	Items       []entities.SecretDraft
}

type CreateOutput struct {
	Data   []entities.Secret `json:"data,omitempty"`
	Errors []entities.Error  `json:"errors,omitempty"`
}

// UseCase

// Perform cria os segredos na tabela do membro. Cada item é independente: um
// inválido não impede os outros (a resposta é 207). O valor é cifrado antes de
// tocar o banco, e nem a resposta nem os erros o repetem. A tabela tem teto
// (entities.MaxSecretsPerTable), conferido a cada item.
func (u CreateSecretUseCase) Perform(ctx context.Context, input CreateInput) CreateOutput {

	owner, denied := authorizeOwner(u.Guard, ctx, input.Requester, input.Application, input.Member, entities.ActionSecretWrite)
	if denied != nil {
		return CreateOutput{Errors: []entities.Error{*denied}}
	}

	if len(input.Items) > MaxCreateBatch {
		return CreateOutput{Errors: []entities.Error{entities.ValidationError("Too many secrets in one request.")}}
	}

	var errs []entities.Error
	data := make([]entities.Secret, 0, len(input.Items))

	for _, item := range input.Items {
		write, failed := u.prepare(ctx, input, owner, item)
		if failed != nil {
			errs = append(errs, *failed)
			continue
		}

		if stored, err := u.Counter.Count(ctx, input.Application, owner); err != nil {
			errs = append(errs, *failure(err))
			continue
		} else if stored >= entities.MaxSecretsPerTable {
			errs = append(errs, *failure(errTableFull))
			continue
		}

		entry := entities.NewAuditLog(input.Requester, input.Application, entities.AuditSecretCreated, entities.AuditAllowed,
			entities.AuditTarget{Type: "secret", ID: write.ID.String()},
			map[string]any{"name": write.Name, "type": string(write.Type), "source": string(write.Source), "member_id": owner.String()})

		var created entities.Secret
		err := u.Tx.Run(ctx, entry, func(ctx context.Context) (err error) {
			created, err = u.Creator.Create(ctx, write)
			return err
		})
		if err != nil {
			errs = append(errs, *failure(err))
			continue
		}

		data = append(data, created)
	}

	if len(data) == 0 {
		data = nil
	}

	return CreateOutput{Data: data, Errors: errs}
}

// prepare valida o item e monta o que o repositório grava, com o valor já
// cifrado. Os erros apontam o campo e o motivo, nunca o valor.
func (u CreateSecretUseCase) prepare(ctx context.Context, input CreateInput, owner uuid.UUID, item entities.SecretDraft) (entities.SecretWrite, *entities.Error) {

	if err := entities.ValidateSecretName(item.Name); err != nil {
		return entities.SecretWrite{}, invalid("name", err.Error())
	}
	if !item.Type.Valid() {
		return entities.SecretWrite{}, invalid("type", "unknown secret type")
	}

	source := item.Source
	if source == "" {
		source = entities.SourceStatic
	}

	creator := input.Requester.Actor.UserID

	switch source {
	case entities.SourceStatic:
		if err := entities.ValidateSecretValue(item.Type, item.Value); err != nil {
			return entities.SecretWrite{}, invalid("value", err.Error())
		}
		if item.ReferenceSecretID != nil {
			return entities.SecretWrite{}, invalid("reference_secret_id", "only a reference secret has a target")
		}

		secret := entities.NewSecret(input.Application, owner, item.Name, item.Type, source, nil, creator)
		versionID := uuid.NewV7()
		ciphertext, err := u.Values.Seal(versionID, item.Value)
		if err != nil {
			internal := entities.InternalServerError(err, nil)
			return entities.SecretWrite{}, &internal
		}
		return entities.SecretWrite{Secret: secret, VersionID: versionID, Ciphertext: ciphertext, Author: creator}, nil

	case entities.SourceReference:
		if item.Value != "" {
			return entities.SecretWrite{}, invalid("value", "a reference has no value of its own")
		}
		if item.ReferenceSecretID == nil {
			return entities.SecretWrite{}, invalid("reference_secret_id", "is required")
		}
		if failed := u.checkReference(ctx, input, item); failed != nil {
			return entities.SecretWrite{}, failed
		}

		return entities.SecretWrite{Secret: entities.NewSecret(input.Application, owner, item.Name, item.Type, source, item.ReferenceSecretID, creator)}, nil
	}

	return entities.SecretWrite{}, invalid("source", "only static and reference are available")
}

// checkReference confere que o alvo é um segredo estático ativo da mesma
// aplicação, do mesmo tipo, e que o ator pode revelá-lo: apontar para o segredo
// de outro membro é ler esse segredo, então exige a mesma permissão do reveal.
// Referência para referência é recusada, o que impede ciclos.
func (u CreateSecretUseCase) checkReference(ctx context.Context, input CreateInput, item entities.SecretDraft) *entities.Error {

	target, err := u.Finder.FindById(ctx, input.Application, *item.ReferenceSecretID)
	if err != nil {
		return invalid("reference_secret_id", "must be an active secret of this application")
	}

	reveal := u.Guard.Perform(ctx, entities.AuthorizeInput{Requester: input.Requester, Application: input.Application, Action: entities.ActionSecretReveal, Owner: target.MemberID})
	if reveal.Error != nil {
		return invalid("reference_secret_id", "must be an active secret of this application")
	}

	if target.Source != entities.SourceStatic {
		return invalid("reference_secret_id", "must point to a static secret")
	}
	if target.Type != item.Type {
		return invalid("type", "must be the same type as the referenced secret")
	}

	return nil
}
