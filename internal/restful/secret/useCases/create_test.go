package useCases_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
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
	t.Run("Quando a contagem da tabela falha.", func(t *testing.T) { whenCreateCountFails(t) })
	t.Run("Quando o segredo vem sem origem.", func(t *testing.T) { whenCreateHasNoSource(t) })
}

// Functions

func whenCreateBuildSucceeds(t *testing.T) {
	r := mocks.NewRig()

	useCase := createUseCase(r)

	assert.Equal(t, r.Secrets, useCase.Repository)
	assert.Equal(t, r.Values, useCase.Values)
	assert.Equal(t, r.Guard, useCase.Guard)
	assert.Equal(t, r.Tx, useCase.Tx)
}

func whenCreateStaticSucceeds(t *testing.T) {
	r := mocks.NewRig()
	draft := mocks.NewDraft()

	output := createUseCase(r).Perform(context.Background(), createInput(r, draft))

	assert.Empty(t, output.Errors)
	if assert.Len(t, output.Data, 1) {
		assert.Equal(t, draft.Name, output.Data[0].Name)
		assert.Equal(t, 1, output.Data[0].Version)
		assert.Equal(t, r.Owner, output.Data[0].MemberID, "a tabela é a do dono resolvido")
	}
	assert.Equal(t, []string{draft.Value}, r.Values.Sealed, "o valor passou pela cifragem")
	if assert.Len(t, r.Secrets.Created, 1) {
		assert.Equal(t, []byte("sealed:"+draft.Value), r.Secrets.Created[0].Ciphertext, "o repositório só recebe o ciphertext")
		assert.Nil(t, r.Secrets.Created[0].ReferenceSecretID, "estático não aponta para nada")
	}
	assert.Equal(t, []entities.Action{entities.ActionSecretWrite}, r.Guard.Actions())
	if assert.Len(t, r.Tx.Entries, 1) {
		assert.NotContains(t, string(r.Tx.Entries[0].Metadata), draft.Value, "a trilha nunca leva o valor")
	}
}

func whenCreateBatchSucceeds(t *testing.T) {
	r := mocks.NewRig()
	drafts := []entities.SecretDraft{mocks.NewDraft(), mocks.NewDraft(), mocks.NewDraft()}

	output := createUseCase(r).Perform(context.Background(), createInput(r, drafts...))

	assert.Empty(t, output.Errors)
	if assert.Len(t, output.Data, 3) {
		for i, draft := range drafts {
			assert.Equal(t, draft.Name, output.Data[i].Name, "a ordem do lote é preservada")
		}
	}
	assert.Equal(t, 3, r.Tx.Runs)
}

func whenCreateBatchIsMixed(t *testing.T) {
	r := mocks.NewRig()
	bad := mocks.NewDraft()
	bad.Name = "minúsculo"
	duplicate := mocks.NewDraft()
	creator := r.Secrets.CreateFunc
	r.Secrets.CreateFunc = func(ctx context.Context, write entities.SecretWrite) (entities.Secret, error) {
		if write.Name == duplicate.Name {
			return entities.Secret{}, mocks.NewUniqueViolation()
		}
		return creator(ctx, write)
	}
	broken := mocks.NewDraft()
	r.Values.SealErr = nil

	output := createUseCase(r).Perform(context.Background(), createInput(r, mocks.NewDraft(), bad, duplicate, broken))

	assert.Len(t, output.Data, 2, "Data só com os sucessos")
	if assert.Len(t, output.Errors, 2) {
		assert.Equal(t, entities.ErrTypeValidation, output.Errors[0].Type, "um erro por item que falhou, na ordem")
		assert.Equal(t, entities.ErrTypeConflict, output.Errors[1].Type)
	}
}

func whenCreateDataIsInvalid(t *testing.T) {
	r := mocks.NewRig()
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

		output := createUseCase(r).Perform(context.Background(), createInput(r, draft))

		assert.Nil(t, output.Data, name)
		if assert.Len(t, output.Errors, 1, name) {
			assert.Equal(t, entities.ErrTypeValidation, output.Errors[0].Type, name)
			assert.Equal(t, http.StatusBadRequest, output.Errors[0].Status, name)
			assert.NotContains(t, output.Errors[0].Detail, secretValue, name)
		}
	}
	assert.Empty(t, r.Secrets.Created, "inválido, o repositório não é chamado")
	assert.Empty(t, r.Values.Sealed, "e nada é cifrado")
}

func whenCreateReferenceSucceeds(t *testing.T) {
	r := mocks.NewRig()
	target := mocks.NewSecret()
	r.Secrets.FindByIDFunc = func(context.Context, uuid.UUID, uuid.UUID) (entities.Secret, error) { return target, nil }
	draft := entities.SecretDraft{Name: "ALIAS", Type: target.Type, Source: entities.SourceReference, ReferenceSecretID: &target.ID}

	output := createUseCase(r).Perform(context.Background(), createInput(r, draft))

	assert.Empty(t, output.Errors)
	if assert.Len(t, output.Data, 1) {
		assert.Equal(t, entities.SourceReference, output.Data[0].Source)
		assert.Zero(t, output.Data[0].Version, "uma referência não tem versão própria")
	}
	assert.Empty(t, r.Values.Sealed, "uma referência não tem valor")
	assert.Nil(t, r.Secrets.Created[0].Ciphertext)
	assert.Equal(t, []entities.Action{entities.ActionSecretWrite, entities.ActionSecretReveal}, r.Guard.Actions(), "apontar para um segredo exige poder revelá-lo")
}

func whenCreateReferenceIsInvalid(t *testing.T) {
	target := mocks.NewSecret()
	reference := mocks.NewSecret()
	reference.Source = entities.SourceReference

	cases := map[string]func(mocks.Rig, entities.SecretDraft) entities.SecretDraft{
		"sem alvo":  func(_ mocks.Rig, d entities.SecretDraft) entities.SecretDraft { d.ReferenceSecretID = nil; return d },
		"com valor": func(_ mocks.Rig, d entities.SecretDraft) entities.SecretDraft { d.Value = "valor"; return d },
		"alvo inexistente": func(r mocks.Rig, d entities.SecretDraft) entities.SecretDraft {
			r.Secrets.FindByIDFunc = func(context.Context, uuid.UUID, uuid.UUID) (entities.Secret, error) {
				return entities.Secret{}, errors.New("not found")
			}
			return d
		},
		"alvo que é referência": func(r mocks.Rig, d entities.SecretDraft) entities.SecretDraft {
			r.Secrets.FindByIDFunc = func(context.Context, uuid.UUID, uuid.UUID) (entities.Secret, error) { return reference, nil }
			return d
		},
		"tipo diferente": func(_ mocks.Rig, d entities.SecretDraft) entities.SecretDraft {
			d.Type = entities.SecretAPIKey
			return d
		},
		"alvo que não pode revelar": func(r mocks.Rig, d entities.SecretDraft) entities.SecretDraft {
			r.Guard.PerformFunc = func(_ context.Context, in entities.AuthorizeInput) entities.AuthorizeOutput {
				if in.Action == entities.ActionSecretReveal {
					forbidden := entities.ForbiddenError()
					return entities.AuthorizeOutput{Error: &forbidden}
				}
				return entities.AuthorizeOutput{Owner: r.Owner}
			}
			return d
		},
	}
	for name, mutate := range cases {
		r := mocks.NewRig()
		r.Secrets.FindByIDFunc = func(context.Context, uuid.UUID, uuid.UUID) (entities.Secret, error) { return target, nil }
		draft := mutate(r, entities.SecretDraft{Name: "ALIAS", Type: target.Type, Source: entities.SourceReference, ReferenceSecretID: &target.ID})

		output := createUseCase(r).Perform(context.Background(), createInput(r, draft))

		assert.Nil(t, output.Data, name)
		if assert.Len(t, output.Errors, 1, name) {
			assert.Equal(t, http.StatusBadRequest, output.Errors[0].Status, name)
		}
		assert.Empty(t, r.Secrets.Created, name)
	}
}

func whenCreateBatchIsTooBig(t *testing.T) {
	r := mocks.NewRig()
	drafts := make([]entities.SecretDraft, useCases.MaxCreateBatch+1)
	for i := range drafts {
		drafts[i] = mocks.NewDraft()
	}

	output := createUseCase(r).Perform(context.Background(), createInput(r, drafts...))

	assert.Nil(t, output.Data)
	assert.Len(t, output.Errors, 1)
	assert.Empty(t, r.Secrets.Created)
}

func whenCreateInputIsEmpty(t *testing.T) {
	r := mocks.NewRig()

	output := createUseCase(r).Perform(context.Background(), createInput(r))

	assert.Nil(t, output.Data)
	assert.Empty(t, output.Errors)
}

func whenCreateNameExists(t *testing.T) {
	r := mocks.NewRig()
	r.Secrets.CreateFunc = func(context.Context, entities.SecretWrite) (entities.Secret, error) {
		return entities.Secret{}, mocks.NewUniqueViolation()
	}

	output := createUseCase(r).Perform(context.Background(), createInput(r, mocks.NewDraft()))

	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, http.StatusConflict, output.Errors[0].Status)
	}
	assert.Nil(t, output.Data)
}

func whenCreateTableIsFull(t *testing.T) {
	r := mocks.NewRig()
	r.Secrets.CountFunc = func(context.Context, uuid.UUID, uuid.UUID) (int, error) { return entities.MaxSecretsPerTable, nil }
	input := createInput(r, mocks.NewDraft())

	output := createUseCase(r).Perform(context.Background(), input)

	assert.Nil(t, output.Data)
	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, http.StatusConflict, output.Errors[0].Status)
	}
	assert.Empty(t, r.Secrets.Created, "tabela cheia, nada é gravado")
	assert.Equal(t, r.Owner, r.Secrets.Owner(), "conta a tabela do dono resolvido")
}

func whenCreateIsDenied(t *testing.T) {
	r := mocks.NewRig()
	r.Guard.Deny(entities.ForbiddenError())

	output := createUseCase(r).Perform(context.Background(), createInput(r, mocks.NewDraft()))

	assert.Nil(t, output.Data)
	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, http.StatusForbidden, output.Errors[0].Status)
	}
	assert.Empty(t, r.Values.Sealed, "negado, nem cifra")
	assert.Empty(t, r.Secrets.Created)
}

func whenCreateInfrastructureFails(t *testing.T) {
	for name, setup := range map[string]func(mocks.Rig){
		"cifragem": func(r mocks.Rig) { r.Values.SealErr = errors.New("segredo-interno") },
		"banco": func(r mocks.Rig) {
			r.Secrets.CreateFunc = func(context.Context, entities.SecretWrite) (entities.Secret, error) {
				return entities.Secret{}, errors.New("segredo-interno")
			}
		},
		"auditoria": func(r mocks.Rig) { r.Tx.AuditErr = errors.New("segredo-interno") },
	} {
		r := mocks.NewRig()
		setup(r)

		output := createUseCase(r).Perform(context.Background(), createInput(r, mocks.NewDraft()))

		assert.Nil(t, output.Data, name)
		if assert.Len(t, output.Errors, 1, name) {
			assert.Equal(t, entities.ErrTypeInternal, output.Errors[0].Type, name)
			assert.NotContains(t, output.Errors[0].Detail, "segredo-interno", name)
		}
	}
}

// Helpers

func createUseCase(r mocks.Rig) useCases.CreateSecretUseCase {
	return useCases.NewCreateSecretUseCase(r.Secrets, r.Values, r.Guard, r.Tx)
}

func createInput(r mocks.Rig, items ...entities.SecretDraft) useCases.CreateInput {
	return useCases.CreateInput{Requester: member_mocks.NewUserRequester(), Application: uuid.NewV7(), Member: entities.SelfOwner, Items: items}
}

func whenCreateCountFails(t *testing.T) {
	r := mocks.NewRig()
	r.Secrets.CountFunc = func(context.Context, uuid.UUID, uuid.UUID) (int, error) { return 0, errors.New("segredo-interno") }

	output := createUseCase(r).Perform(context.Background(), createInput(r, mocks.NewDraft()))

	assert.Nil(t, output.Data)
	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, entities.ErrTypeInternal, output.Errors[0].Type)
		assert.NotContains(t, output.Errors[0].Detail, "segredo-interno")
	}
	assert.Empty(t, r.Secrets.Created, "sem poder contar, não grava")
}

func whenCreateHasNoSource(t *testing.T) {
	r := mocks.NewRig()
	draft := mocks.NewDraft()
	draft.Source = ""

	output := createUseCase(r).Perform(context.Background(), createInput(r, draft))

	assert.Empty(t, output.Errors)
	if assert.Len(t, output.Data, 1) {
		assert.Equal(t, entities.SourceStatic, output.Data[0].Source, "origem vazia vale como estático")
	}
}
