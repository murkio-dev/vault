package useCases_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/secret/mocks"
	"github.com/murkio-dev/vault/internal/restful/secret/useCases"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestCreateSecret(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenCreateBuildSucceeds(t) })
	t.Run("Quando cria um segredo estático.", func(t *testing.T) { whenCreateStaticSucceeds(t) })
	t.Run("Quando cria um lote válido.", func(t *testing.T) { whenCreateBatchSucceeds(t) })
	t.Run("Quando o lote é misto.", func(t *testing.T) { whenCreateBatchIsMixed(t) })
	t.Run("Quando os dados são inválidos.", func(t *testing.T) { whenCreateDataIsInvalid(t) })
	t.Run("Quando cria uma referência.", func(t *testing.T) { whenCreateReferenceSucceeds(t) })
	t.Run("Quando a referência é inválida.", func(t *testing.T) { whenCreateReferenceIsInvalid(t) })
	t.Run("Quando o lote passa do teto.", func(t *testing.T) { whenCreateBatchIsTooBig(t) })
	t.Run("Quando a lista de entrada é vazia.", func(t *testing.T) { whenCreateInputIsEmpty(t) })
	t.Run("Quando o nome já existe.", func(t *testing.T) { whenCreateNameExists(t) })
	t.Run("Quando a tabela do membro está cheia.", func(t *testing.T) { whenCreateTableIsFull(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenCreateIsDenied(t) })
	t.Run("Quando a cifragem, o banco ou a auditoria falham.", func(t *testing.T) { whenCreateInfrastructureFails(t) })
}

// Functions

func (r *rig) create() useCases.CreateSecretUseCase {
	return useCases.NewCreateSecretUseCase(r.secrets, r.secrets, r.secrets, r.values, r.guard, r.tx)
}

func (r *rig) createInput(items ...entities.SecretDraft) useCases.CreateInput {
	return useCases.CreateInput{Requester: r.requester(), Application: uuid.NewV7(), Member: entities.SelfOwner, Items: items}
}

func whenCreateBuildSucceeds(t *testing.T) {
	r := newRig()

	useCase := r.create()

	assert.Equal(t, r.secrets, useCase.Creator)
	assert.Equal(t, r.secrets, useCase.Counter)
	assert.Equal(t, r.secrets, useCase.Finder)
	assert.Equal(t, r.values, useCase.Values)
	assert.Equal(t, r.guard, useCase.Guard)
	assert.Equal(t, r.tx, useCase.Tx)
}

func whenCreateStaticSucceeds(t *testing.T) {
	r := newRig()
	draft := mocks.NewDraft()

	output := r.create().Perform(context.Background(), r.createInput(draft))

	assert.Empty(t, output.Errors)
	if assert.Len(t, output.Data, 1) {
		assert.Equal(t, draft.Name, output.Data[0].Name)
		assert.Equal(t, 1, output.Data[0].Version)
		assert.Equal(t, r.owner, output.Data[0].MemberID, "a tabela é a do dono resolvido")
	}
	assert.Equal(t, []string{draft.Value}, r.values.Sealed, "o valor passou pela cifragem")
	if assert.Len(t, r.secrets.Created, 1) {
		assert.Equal(t, []byte("sealed:"+draft.Value), r.secrets.Created[0].Ciphertext, "o repositório só recebe o ciphertext")
		assert.Nil(t, r.secrets.Created[0].ReferenceSecretID, "estático não aponta para nada")
	}
	assert.Equal(t, []entities.Action{entities.ActionSecretWrite}, r.guard.Actions())
	if assert.Len(t, r.tx.Entries, 1) {
		assert.NotContains(t, string(r.tx.Entries[0].Metadata), draft.Value, "a trilha nunca leva o valor")
	}
}

func whenCreateBatchSucceeds(t *testing.T) {
	r := newRig()
	drafts := []entities.SecretDraft{mocks.NewDraft(), mocks.NewDraft(), mocks.NewDraft()}

	output := r.create().Perform(context.Background(), r.createInput(drafts...))

	assert.Empty(t, output.Errors)
	if assert.Len(t, output.Data, 3) {
		for i, draft := range drafts {
			assert.Equal(t, draft.Name, output.Data[i].Name, "a ordem do lote é preservada")
		}
	}
	assert.Equal(t, 3, r.tx.Runs)
}

func whenCreateBatchIsMixed(t *testing.T) {
	r := newRig()
	bad := mocks.NewDraft()
	bad.Name = "minúsculo"
	duplicate := mocks.NewDraft()
	creator := r.secrets.CreateFunc
	r.secrets.CreateFunc = func(ctx context.Context, write entities.SecretWrite) (entities.Secret, error) {
		if write.Name == duplicate.Name {
			return entities.Secret{}, mocks.NewUniqueViolation()
		}
		return creator(ctx, write)
	}
	broken := mocks.NewDraft()
	r.values.SealErr = nil

	output := r.create().Perform(context.Background(), r.createInput(mocks.NewDraft(), bad, duplicate, broken))

	assert.Len(t, output.Data, 2, "Data só com os sucessos")
	if assert.Len(t, output.Errors, 2) {
		assert.Equal(t, entities.ErrTypeValidation, output.Errors[0].Type, "um erro por item que falhou, na ordem")
		assert.Equal(t, entities.ErrTypeConflict, output.Errors[1].Type)
	}
}

func whenCreateDataIsInvalid(t *testing.T) {
	r := newRig()
	secretValue := "valor-que-nao-pode-vazar"

	for name, mutate := range map[string]func(*entities.SecretDraft){
		"nome vazio":        func(d *entities.SecretDraft) { d.Name = "" },
		"nome minúsculo":    func(d *entities.SecretDraft) { d.Name = "database_url" },
		"nome com espaço":   func(d *entities.SecretDraft) { d.Name = "DATABASE URL" },
		"nome enorme":       func(d *entities.SecretDraft) { d.Name = "A" + strings.Repeat("B", 128) },
		"tipo desconhecido": func(d *entities.SecretDraft) { d.Type = "nope" },
		"valor vazio":       func(d *entities.SecretDraft) { d.Value = "" },
		"valor gigante":     func(d *entities.SecretDraft) { d.Value = strings.Repeat("a", entities.MaxSecretValueBytes+1) },
		"url sem host":      func(d *entities.SecretDraft) { d.Type, d.Value = entities.SecretURL, secretValue },
		"origem reservada":  func(d *entities.SecretDraft) { d.Source = entities.SourceGenerated },
		"origem dinâmica":   func(d *entities.SecretDraft) { d.Source = entities.SourceDynamic },
		"origem inventada":  func(d *entities.SecretDraft) { d.Source = "magica" },
		"alvo em estático":  func(d *entities.SecretDraft) { id := uuid.NewV7(); d.ReferenceSecretID = &id },
	} {
		draft := mocks.NewDraft()
		mutate(&draft)

		output := r.create().Perform(context.Background(), r.createInput(draft))

		assert.Nil(t, output.Data, name)
		if assert.Len(t, output.Errors, 1, name) {
			assert.Equal(t, entities.ErrTypeValidation, output.Errors[0].Type, name)
			assert.Equal(t, http.StatusBadRequest, output.Errors[0].Status, name)
			assert.NotContains(t, output.Errors[0].Detail, secretValue, name)
		}
	}
	assert.Empty(t, r.secrets.Created, "inválido, o repositório não é chamado")
	assert.Empty(t, r.values.Sealed, "e nada é cifrado")
}

func whenCreateReferenceSucceeds(t *testing.T) {
	r := newRig()
	target := mocks.NewSecret()
	r.secrets.FindByIDFunc = func(context.Context, uuid.UUID, uuid.UUID) (entities.Secret, error) { return target, nil }
	draft := entities.SecretDraft{Name: "ALIAS", Type: target.Type, Source: entities.SourceReference, ReferenceSecretID: &target.ID}

	output := r.create().Perform(context.Background(), r.createInput(draft))

	assert.Empty(t, output.Errors)
	if assert.Len(t, output.Data, 1) {
		assert.Equal(t, entities.SourceReference, output.Data[0].Source)
		assert.Zero(t, output.Data[0].Version, "uma referência não tem versão própria")
	}
	assert.Empty(t, r.values.Sealed, "uma referência não tem valor")
	assert.Nil(t, r.secrets.Created[0].Ciphertext)
	assert.Equal(t, []entities.Action{entities.ActionSecretWrite, entities.ActionSecretReveal}, r.guard.Actions(), "apontar para um segredo exige poder revelá-lo")
}

func whenCreateReferenceIsInvalid(t *testing.T) {
	target := mocks.NewSecret()
	reference := mocks.NewSecret()
	reference.Source = entities.SourceReference

	cases := map[string]func(*rig, *entities.SecretDraft){
		"sem alvo":  func(_ *rig, d *entities.SecretDraft) { d.ReferenceSecretID = nil },
		"com valor": func(_ *rig, d *entities.SecretDraft) { d.Value = "valor" },
		"alvo inexistente": func(r *rig, _ *entities.SecretDraft) {
			r.secrets.FindByIDFunc = func(context.Context, uuid.UUID, uuid.UUID) (entities.Secret, error) {
				return entities.Secret{}, errors.New("not found")
			}
		},
		"alvo que é referência": func(r *rig, _ *entities.SecretDraft) {
			r.secrets.FindByIDFunc = func(context.Context, uuid.UUID, uuid.UUID) (entities.Secret, error) { return reference, nil }
		},
		"tipo diferente": func(_ *rig, d *entities.SecretDraft) { d.Type = entities.SecretAPIKey },
		"alvo que não pode revelar": func(r *rig, _ *entities.SecretDraft) {
			r.guard.PerformFunc = func(_ context.Context, in entities.AuthorizeInput) entities.AuthorizeOutput {
				if in.Action == entities.ActionSecretReveal {
					forbidden := entities.ForbiddenError()
					return entities.AuthorizeOutput{Error: &forbidden}
				}
				return entities.AuthorizeOutput{Owner: r.owner}
			}
		},
	}
	for name, mutate := range cases {
		r := newRig()
		r.secrets.FindByIDFunc = func(context.Context, uuid.UUID, uuid.UUID) (entities.Secret, error) { return target, nil }
		draft := entities.SecretDraft{Name: "ALIAS", Type: target.Type, Source: entities.SourceReference, ReferenceSecretID: &target.ID}
		mutate(r, &draft)

		output := r.create().Perform(context.Background(), r.createInput(draft))

		assert.Nil(t, output.Data, name)
		if assert.Len(t, output.Errors, 1, name) {
			assert.Equal(t, http.StatusBadRequest, output.Errors[0].Status, name)
		}
		assert.Empty(t, r.secrets.Created, name)
	}
}

func whenCreateBatchIsTooBig(t *testing.T) {
	r := newRig()
	drafts := make([]entities.SecretDraft, useCases.MaxCreateBatch+1)
	for i := range drafts {
		drafts[i] = mocks.NewDraft()
	}

	output := r.create().Perform(context.Background(), r.createInput(drafts...))

	assert.Nil(t, output.Data)
	assert.Len(t, output.Errors, 1)
	assert.Empty(t, r.secrets.Created)
}

func whenCreateInputIsEmpty(t *testing.T) {
	r := newRig()

	output := r.create().Perform(context.Background(), r.createInput())

	assert.Nil(t, output.Data)
	assert.Empty(t, output.Errors)
}

func whenCreateNameExists(t *testing.T) {
	r := newRig()
	r.secrets.CreateFunc = func(context.Context, entities.SecretWrite) (entities.Secret, error) {
		return entities.Secret{}, mocks.NewUniqueViolation()
	}

	output := r.create().Perform(context.Background(), r.createInput(mocks.NewDraft()))

	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, http.StatusConflict, output.Errors[0].Status)
	}
	assert.Nil(t, output.Data)
}

func whenCreateTableIsFull(t *testing.T) {
	r := newRig()
	r.secrets.CountFunc = func(context.Context, uuid.UUID, uuid.UUID) (int, error) { return entities.MaxSecretsPerTable, nil }
	input := r.createInput(mocks.NewDraft())

	output := r.create().Perform(context.Background(), input)

	assert.Nil(t, output.Data)
	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, http.StatusConflict, output.Errors[0].Status)
	}
	assert.Empty(t, r.secrets.Created, "tabela cheia, nada é gravado")
	assert.Equal(t, r.owner, r.secrets.Owner(), "conta a tabela do dono resolvido")
}

func whenCreateIsDenied(t *testing.T) {
	r := newRig()
	r.guard.Deny(entities.ForbiddenError())

	output := r.create().Perform(context.Background(), r.createInput(mocks.NewDraft()))

	assert.Nil(t, output.Data)
	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, http.StatusForbidden, output.Errors[0].Status)
	}
	assert.Empty(t, r.values.Sealed, "negado, nem cifra")
	assert.Empty(t, r.secrets.Created)
}

func whenCreateInfrastructureFails(t *testing.T) {
	for name, setup := range map[string]func(*rig){
		"cifragem": func(r *rig) { r.values.SealErr = errors.New("segredo-interno") },
		"banco": func(r *rig) {
			r.secrets.CreateFunc = func(context.Context, entities.SecretWrite) (entities.Secret, error) {
				return entities.Secret{}, errors.New("segredo-interno")
			}
		},
		"auditoria": func(r *rig) { r.tx.AuditErr = errors.New("segredo-interno") },
	} {
		r := newRig()
		setup(r)

		output := r.create().Perform(context.Background(), r.createInput(mocks.NewDraft()))

		assert.Nil(t, output.Data, name)
		if assert.Len(t, output.Errors, 1, name) {
			assert.Equal(t, entities.ErrTypeInternal, output.Errors[0].Type, name)
			assert.NotContains(t, output.Errors[0].Detail, "segredo-interno", name)
		}
	}
}
