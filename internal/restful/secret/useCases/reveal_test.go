package useCases_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/secret/mocks"
	"github.com/murkio-dev/vault/internal/restful/secret/useCases"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestRevealSecret(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenRevealBuildSucceeds(t) })
	t.Run("Quando revela a versão atual.", func(t *testing.T) { whenRevealSucceeds(t) })
	t.Run("Quando revela uma versão antiga.", func(t *testing.T) { whenRevealAnOldVersion(t) })
	t.Run("Quando revela uma referência.", func(t *testing.T) { whenRevealAReference(t) })
	t.Run("Quando a versão é negativa.", func(t *testing.T) { whenRevealVersionIsNegative(t) })
	t.Run("Quando o segredo ou a versão não existem.", func(t *testing.T) { whenRevealIsMissing(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenRevealIsDenied(t) })
	t.Run("Quando a auditoria falha.", func(t *testing.T) { whenRevealAuditFails(t) })
	t.Run("Quando a decifragem falha.", func(t *testing.T) { whenRevealDecryptionFails(t) })
}

// Functions

func (r *rig) reveal() useCases.RevealSecretUseCase {
	return useCases.NewRevealSecretUseCase(r.secrets, r.values, r.guard, r.tx)
}

func (r *rig) revealInput(version int) useCases.RevealInput {
	return useCases.RevealInput{Requester: r.requester(), Application: uuid.NewV7(), Member: entities.SelfOwner, Secret: uuid.NewV7(), Version: version}
}

func whenRevealBuildSucceeds(t *testing.T) {
	r := newRig()

	useCase := r.reveal()

	assert.Equal(t, r.secrets, useCase.OwnerFinder)
	assert.Equal(t, r.values, useCase.Values)
	assert.Equal(t, r.tx, useCase.Tx)
}

func whenRevealSucceeds(t *testing.T) {
	r := newRig()

	output := r.reveal().Perform(context.Background(), r.revealInput(0))

	assert.Nil(t, output.Error)
	assert.Equal(t, "valor-revelado", output.Data.Value)
	assert.Equal(t, 1, output.Data.Version)
	assert.Equal(t, []entities.Action{entities.ActionSecretReveal}, r.guard.Actions())
	if assert.Len(t, r.tx.Entries, 1) {
		assert.Equal(t, entities.AuditSecretRevealed, r.tx.Entries[0].Action)
		assert.NotContains(t, string(r.tx.Entries[0].Metadata), "valor-revelado", "a trilha leva nome e versão, nunca o valor")
	}
}

func whenRevealAnOldVersion(t *testing.T) {
	r := newRig()

	output := r.reveal().Perform(context.Background(), r.revealInput(3))

	assert.Nil(t, output.Error)
	assert.Equal(t, 3, output.Data.Version)
	assert.Contains(t, string(r.tx.Entries[0].Metadata), `"version":3`)
}

func whenRevealAReference(t *testing.T) {
	r := newRig()
	target := uuid.NewV7()
	r.secrets.FindByOwnerFunc = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (entities.Secret, error) {
		secret := mocks.NewSecret()
		secret.Source, secret.ReferenceSecretID = entities.SourceReference, &target
		return secret, nil
	}

	output := r.reveal().Perform(context.Background(), r.revealInput(0))

	assert.Nil(t, output.Error)
	assert.Contains(t, string(r.tx.Entries[0].Metadata), target.String(), "a trilha diz de onde o valor foi resolvido")
}

func whenRevealVersionIsNegative(t *testing.T) {
	r := newRig()

	output := r.reveal().Perform(context.Background(), r.revealInput(-1))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusBadRequest, output.Error.Status)
	}
	assert.Zero(t, r.guard.Calls)
}

func whenRevealIsMissing(t *testing.T) {
	missingSecret := newRig()
	missingSecret.secrets.FindByOwnerFunc = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (entities.Secret, error) {
		return entities.Secret{}, sql.ErrNoRows
	}
	missingVersion := newRig()
	missingVersion.values.ValueErr = sql.ErrNoRows

	for name, r := range map[string]*rig{"segredo": missingSecret, "versão": missingVersion} {
		output := r.reveal().Perform(context.Background(), r.revealInput(9))

		if assert.NotNil(t, output.Error, name) {
			assert.Equal(t, http.StatusNotFound, output.Error.Status, name)
		}
		assert.Empty(t, output.Data.Value, name)
		assert.Empty(t, r.tx.Entries, name)
	}
}

func whenRevealIsDenied(t *testing.T) {
	r := newRig()
	r.guard.Deny(entities.ForbiddenError())

	output := r.reveal().Perform(context.Background(), r.revealInput(0))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusForbidden, output.Error.Status)
	}
	assert.Empty(t, output.Data.Value, "negado, o valor nunca sai")
	assert.Zero(t, r.tx.Runs)
}

func whenRevealAuditFails(t *testing.T) {
	r := newRig()
	r.tx.AuditErr = errors.New("disk full")

	output := r.reveal().Perform(context.Background(), r.revealInput(0))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
	}
	assert.Empty(t, output.Data.Value, "sem trilha, o valor não sai")
}

func whenRevealDecryptionFails(t *testing.T) {
	r := newRig()
	r.values.ValueErr = errors.New("segredo-interno")

	output := r.reveal().Perform(context.Background(), r.revealInput(0))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
		assert.NotContains(t, output.Error.Detail, "segredo-interno")
	}
	assert.Empty(t, output.Data.Value)
}
