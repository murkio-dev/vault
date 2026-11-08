package useCases_test

import (
	"context"
	"database/sql"
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

func TestWriteSecrets(t *testing.T) {
	t.Run("Quando os usecases buildam corretamente.", func(t *testing.T) { whenWriteBuildSucceeds(t) })
	t.Run("Quando troca o valor.", func(t *testing.T) { whenUpdateSucceeds(t) })
	t.Run("Quando o valor novo é inválido.", func(t *testing.T) { whenUpdateValueIsInvalid(t) })
	t.Run("Quando atualiza uma referência.", func(t *testing.T) { whenUpdateTargetsAReference(t) })
	t.Run("Quando a versão corre com outra gravação.", func(t *testing.T) { whenUpdateLosesTheRace(t) })
	t.Run("Quando renomeia.", func(t *testing.T) { whenRenameSucceeds(t) })
	t.Run("Quando o nome novo é inválido ou repetido.", func(t *testing.T) { whenRenameFails(t) })
	t.Run("Quando apaga.", func(t *testing.T) { whenDeleteSucceeds(t) })
	t.Run("Quando outros segredos apontam para o apagado.", func(t *testing.T) { whenDeleteIsReferenced(t) })
	t.Run("Quando o segredo não existe.", func(t *testing.T) { whenWriteSecretIsMissing(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenWriteIsDenied(t) })
	t.Run("Quando a cifragem, o banco ou a auditoria falham.", func(t *testing.T) { whenWriteInfrastructureFails(t) })
}

// Functions

func (r *rig) update() useCases.UpdateSecretUseCase {
	return useCases.NewUpdateSecretUseCase(r.secrets, r.secrets, r.values, r.guard, r.tx)
}

func (r *rig) rename() useCases.RenameSecretUseCase {
	return useCases.NewRenameSecretUseCase(r.secrets, r.secrets, r.guard, r.tx)
}

func (r *rig) delete() useCases.DeleteSecretUseCase {
	return useCases.NewDeleteSecretUseCase(r.secrets, r.secrets, r.secrets, r.guard, r.tx)
}

func (r *rig) updateInput(value string) useCases.UpdateInput {
	return useCases.UpdateInput{Requester: r.requester(), Application: uuid.NewV7(), Member: entities.SelfOwner, Secret: uuid.NewV7(), Value: value}
}

func (r *rig) renameInput(name string) useCases.RenameInput {
	return useCases.RenameInput{Requester: r.requester(), Application: uuid.NewV7(), Member: entities.SelfOwner, Secret: uuid.NewV7(), Name: name}
}

func (r *rig) deleteInput() useCases.DeleteInput {
	return useCases.DeleteInput{Requester: r.requester(), Application: uuid.NewV7(), Member: entities.SelfOwner, Secret: uuid.NewV7()}
}

func whenWriteBuildSucceeds(t *testing.T) {
	r := newRig()

	assert.Equal(t, r.values, r.update().Values)
	assert.Equal(t, r.tx, r.update().Tx)
	assert.Equal(t, r.secrets, r.rename().OwnerFinder)
	assert.Equal(t, r.guard, r.delete().Guard)
}

func whenUpdateSucceeds(t *testing.T) {
	r := newRig()
	value := "novo-valor-nao-vazar"

	output := r.update().Perform(context.Background(), r.updateInput(value))

	assert.Nil(t, output.Error)
	assert.Len(t, r.secrets.Added, 1)
	assert.Equal(t, []string{value}, r.values.Sealed)
	if assert.Len(t, r.tx.Entries, 1) {
		assert.Equal(t, entities.AuditSecretUpdated, r.tx.Entries[0].Action)
		assert.Contains(t, string(r.tx.Entries[0].Metadata), `"version":2`, "a trilha leva a versão nova")
		assert.NotContains(t, string(r.tx.Entries[0].Metadata), value)
	}
}

func whenUpdateValueIsInvalid(t *testing.T) {
	r := newRig()
	r.secrets.FindByOwnerFunc = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (entities.Secret, error) {
		secret := mocks.NewSecret()
		secret.Type = entities.SecretURL
		return secret, nil
	}

	for name, value := range map[string]string{"vazio": "", "gigante": strings.Repeat("a", entities.MaxSecretValueBytes+1), "url sem host": "segredo-que-nao-e-url"} {
		output := r.update().Perform(context.Background(), r.updateInput(value))

		if assert.NotNil(t, output.Error, name) {
			assert.Equal(t, http.StatusBadRequest, output.Error.Status, name)
			assert.NotContains(t, output.Error.Detail, "segredo-que-nao-e-url", name)
		}
	}
	assert.Empty(t, r.secrets.Added)
	assert.Empty(t, r.values.Sealed)
}

func whenUpdateTargetsAReference(t *testing.T) {
	r := newRig()
	r.secrets.FindByOwnerFunc = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (entities.Secret, error) {
		secret := mocks.NewSecret()
		secret.Source = entities.SourceReference
		return secret, nil
	}

	output := r.update().Perform(context.Background(), r.updateInput("valor"))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusBadRequest, output.Error.Status, "uma referência não tem valor próprio")
	}
	assert.Empty(t, r.secrets.Added)
}

func whenUpdateLosesTheRace(t *testing.T) {
	r := newRig()
	r.secrets.AddVersionFunc = func(context.Context, entities.SecretWrite) (entities.Secret, error) {
		return entities.Secret{}, sql.ErrNoRows
	}

	output := r.update().Perform(context.Background(), r.updateInput("valor"))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusNotFound, output.Error.Status, "o UPDATE condicional impede duas versões com o mesmo número")
	}
}

func whenRenameSucceeds(t *testing.T) {
	r := newRig()
	name := "NOVO_NOME"

	output := r.rename().Perform(context.Background(), r.renameInput(name))

	assert.Nil(t, output.Error)
	assert.Equal(t, name, output.Data.Name)
	if assert.Len(t, r.secrets.Renamed, 1) {
		assert.Equal(t, name, r.secrets.Renamed[0].Name)
	}
	assert.Equal(t, []string{entities.AuditSecretRenamed}, r.tx.Actions())
}

func whenRenameFails(t *testing.T) {
	for _, name := range []string{"", "minúsculo", "COM ESPAÇO", "1COMECA_COM_NUMERO"} {
		r := newRig()

		output := r.rename().Perform(context.Background(), r.renameInput(name))

		if assert.NotNil(t, output.Error, name) {
			assert.Equal(t, http.StatusBadRequest, output.Error.Status, name)
		}
		assert.Empty(t, r.secrets.Renamed)
	}

	r := newRig()
	r.secrets.RenameFunc = func(context.Context, entities.SecretWrite) (entities.Secret, error) {
		return entities.Secret{}, mocks.NewUniqueViolation()
	}
	conflict := r.rename().Perform(context.Background(), r.renameInput("JA_EXISTE"))
	if assert.NotNil(t, conflict.Error) {
		assert.Equal(t, http.StatusConflict, conflict.Error.Status)
	}
}

func whenDeleteSucceeds(t *testing.T) {
	r := newRig()

	output := r.delete().Perform(context.Background(), r.deleteInput())

	assert.Nil(t, output.Error)
	assert.Len(t, r.secrets.Deleted, 1)
	assert.Equal(t, []string{entities.AuditSecretDeleted}, r.tx.Actions())
}

func whenDeleteIsReferenced(t *testing.T) {
	r := newRig()
	r.secrets.CountReferencesFunc = func(context.Context, uuid.UUID, uuid.UUID) (int, error) { return 2, nil }

	output := r.delete().Perform(context.Background(), r.deleteInput())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusConflict, output.Error.Status)
	}
	assert.Empty(t, r.secrets.Deleted, "apagar quebraria o runtime de quem aponta")
}

func whenWriteSecretIsMissing(t *testing.T) {
	r := newRig()
	r.secrets.FindByOwnerFunc = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (entities.Secret, error) {
		return entities.Secret{}, sql.ErrNoRows
	}

	update := r.update().Perform(context.Background(), r.updateInput("valor"))
	rename := r.rename().Perform(context.Background(), r.renameInput("OUTRO"))
	remove := r.delete().Perform(context.Background(), r.deleteInput())

	for _, missing := range []*entities.Error{update.Error, rename.Error, remove.Error} {
		if assert.NotNil(t, missing) {
			assert.Equal(t, http.StatusNotFound, missing.Status)
		}
	}
	assert.Zero(t, r.tx.Runs)
}

func whenWriteIsDenied(t *testing.T) {
	r := newRig()
	r.guard.Deny(entities.ForbiddenError())

	update := r.update().Perform(context.Background(), r.updateInput("valor"))
	rename := r.rename().Perform(context.Background(), r.renameInput("OUTRO"))
	remove := r.delete().Perform(context.Background(), r.deleteInput())

	for _, denied := range []*entities.Error{update.Error, rename.Error, remove.Error} {
		if assert.NotNil(t, denied) {
			assert.Equal(t, http.StatusForbidden, denied.Status)
		}
	}
	assert.Zero(t, r.tx.Runs)
	assert.Empty(t, r.values.Sealed, "negado, nem cifra")
}

func whenWriteInfrastructureFails(t *testing.T) {
	boom := errors.New("segredo-interno")
	for name, setup := range map[string]func(*rig){
		"cifragem": func(r *rig) { r.values.SealErr = boom },
		"banco": func(r *rig) {
			r.secrets.AddVersionFunc = func(context.Context, entities.SecretWrite) (entities.Secret, error) { return entities.Secret{}, boom }
		},
		"auditoria": func(r *rig) { r.tx.AuditErr = boom },
	} {
		r := newRig()
		setup(r)

		output := r.update().Perform(context.Background(), r.updateInput("valor"))

		if assert.NotNil(t, output.Error, name) {
			assert.Equal(t, entities.ErrTypeInternal, output.Error.Type, name)
			assert.NotContains(t, output.Error.Detail, "segredo-interno", name)
		}
	}

	r := newRig()
	r.secrets.CountReferencesFunc = func(context.Context, uuid.UUID, uuid.UUID) (int, error) { return 0, boom }
	remove := r.delete().Perform(context.Background(), r.deleteInput())
	if assert.NotNil(t, remove.Error) {
		assert.Equal(t, entities.ErrTypeInternal, remove.Error.Type)
	}
}
