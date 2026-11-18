package useCases_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/murkio-dev/vault/internal/restful/secret/mocks"
	"github.com/murkio-dev/vault/internal/restful/secret/useCases"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestListVersions(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenVersionsBuildSucceeds(t) })
	t.Run("Quando lista as versões.", func(t *testing.T) { whenVersionsSucceed(t) })
	t.Run("Quando o segredo das versões não existe.", func(t *testing.T) { whenVersionsSecretIsMissing(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenVersionsIsDenied(t) })
	t.Run("Quando ocorre erro de banco.", func(t *testing.T) { whenVersionsFailsInDb(t) })
}

// Functions

func whenVersionsBuildSucceeds(t *testing.T) {
	r := mocks.NewRig()

	assert.Equal(t, r.Secrets, versionsUseCase(r).Repository)
	assert.Equal(t, r.Versions, versionsUseCase(r).Versions)
	assert.Equal(t, r.Guard, versionsUseCase(r).Guard)
	assert.NotPanics(t, func() { useCases.NewListVersionsUseCase(nil, nil, nil) })
}

func whenVersionsSucceed(t *testing.T) {
	r := mocks.NewRig()
	input := versionsInput(entities.SelfOwner)
	versions := []entities.SecretVersion{{Version: 2}, {Version: 1}}
	r.Versions.ListFunc = func(context.Context, uuid.UUID, uuid.UUID, int, int) ([]entities.SecretVersion, int, error) {
		return versions, len(versions), nil
	}

	output := versionsUseCase(r).Perform(context.Background(), input)

	assert.Nil(t, output.Error)
	assert.Equal(t, versions, output.Data)
	assert.Equal(t, []uuid.UUID{input.Secret}, r.Versions.Secrets, "lista as versões do segredo achado")
	assert.Equal(t, []uuid.UUID{input.Application}, r.Versions.Applications, "e só da aplicação pedida")
}

func whenVersionsSecretIsMissing(t *testing.T) {
	r := mocks.NewRig()
	r.Secrets.FindByOwnerFunc = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (entities.Secret, error) {
		return entities.Secret{}, sql.ErrNoRows
	}
	r.Versions.ListFunc = func(context.Context, uuid.UUID, uuid.UUID, int, int) ([]entities.SecretVersion, int, error) {
		t.Error("sem o segredo, as versões não são lidas")
		return nil, 0, nil
	}

	output := versionsUseCase(r).Perform(context.Background(), versionsInput(entities.SelfOwner))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusNotFound, output.Error.Status)
	}
}

func whenVersionsIsDenied(t *testing.T) {
	r := mocks.NewRig()
	r.Guard.Deny(entities.ForbiddenError())
	r.Secrets.FindByOwnerFunc = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (entities.Secret, error) {
		t.Error("negado, o repositório não é chamado")
		return entities.Secret{}, nil
	}

	output := versionsUseCase(r).Perform(context.Background(), versionsInput(uuid.NewV7()))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusForbidden, output.Error.Status)
	}
	assert.Empty(t, r.Versions.Secrets, "negado, as versões não são lidas")
}

func whenVersionsFailsInDb(t *testing.T) {
	boom := errors.New("segredo-interno")
	for name, setup := range map[string]func(mocks.Rig){
		"busca do segredo": func(r mocks.Rig) {
			r.Secrets.FindByOwnerFunc = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (entities.Secret, error) {
				return entities.Secret{}, boom
			}
		},
		"lista das versões": func(r mocks.Rig) {
			r.Versions.ListFunc = func(context.Context, uuid.UUID, uuid.UUID, int, int) ([]entities.SecretVersion, int, error) {
				return nil, 0, boom
			}
		},
	} {
		r := mocks.NewRig()
		setup(r)

		output := versionsUseCase(r).Perform(context.Background(), versionsInput(entities.SelfOwner))

		if assert.NotNil(t, output.Error, name) {
			assert.Equal(t, entities.ErrTypeInternal, output.Error.Type, name)
			assert.NotContains(t, output.Error.Detail, "segredo-interno", name)
		}
	}
}

// Helpers

func versionsUseCase(r mocks.Rig) useCases.ListVersionsUseCase {
	return useCases.NewListVersionsUseCase(r.Secrets, r.Versions, r.Guard)
}

func versionsInput(member uuid.UUID) useCases.VersionsInput {
	return useCases.VersionsInput{Requester: member_mocks.NewUserRequester(), Application: uuid.NewV7(), Member: member, Secret: uuid.NewV7()}
}
