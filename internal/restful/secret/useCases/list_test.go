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

func TestReadSecrets(t *testing.T) {
	t.Run("Quando os usecases buildam corretamente.", func(t *testing.T) { whenReadBuildSucceeds(t) })
	t.Run("Quando lista os metadados.", func(t *testing.T) { whenListSucceeds(t) })
	t.Run("Quando a lista está vazia.", func(t *testing.T) { whenListIsEmpty(t) })
	t.Run("Quando busca um segredo.", func(t *testing.T) { whenGetSucceeds(t) })
	t.Run("Quando o segredo não existe.", func(t *testing.T) { whenGetIsMissing(t) })
	t.Run("Quando lista as versões.", func(t *testing.T) { whenVersionsSucceed(t) })
	t.Run("Quando o segredo das versões não existe.", func(t *testing.T) { whenVersionsSecretIsMissing(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenReadIsDenied(t) })
	t.Run("Quando ocorre erro de banco.", func(t *testing.T) { whenReadFailsInDb(t) })
}

// Functions

func (r *rig) list() useCases.ListSecretsUseCase {
	return useCases.NewListSecretsUseCase(r.secrets, r.guard)
}

func (r *rig) get() useCases.GetSecretUseCase {
	return useCases.NewGetSecretUseCase(r.secrets, r.guard)
}

func (r *rig) listVersions() useCases.ListVersionsUseCase {
	return useCases.NewListVersionsUseCase(r.secrets, r.versions, r.guard)
}

func whenReadBuildSucceeds(t *testing.T) {
	r := newRig()

	assert.Equal(t, r.secrets, r.list().Lister)
	assert.Equal(t, r.guard, r.list().Guard)
	assert.Equal(t, r.secrets, r.get().OwnerFinder)
	assert.Equal(t, r.secrets, r.listVersions().OwnerFinder)
}

func whenListSucceeds(t *testing.T) {
	r := newRig()
	secrets := []entities.Secret{mocks.NewSecret(), mocks.NewSecret()}
	r.secrets.ListFunc = func(_ context.Context, _, owner uuid.UUID, _, _ int) ([]entities.Secret, int, error) {
		assert.Equal(t, r.owner, owner, "lista a tabela do dono resolvido")
		return secrets, len(secrets), nil
	}

	output := r.list().Perform(context.Background(), useCases.ListInput{Requester: r.requester(), Application: uuid.NewV7(), Member: entities.SelfOwner})

	assert.Nil(t, output.Error)
	assert.Equal(t, secrets, output.Data)
	assert.Equal(t, []entities.Action{entities.ActionSecretRead}, r.guard.Actions())
	assert.True(t, r.guard.Inputs[0].OwnerMustExist)
}

func whenListIsEmpty(t *testing.T) {
	r := newRig()

	output := r.list().Perform(context.Background(), useCases.ListInput{Requester: r.requester(), Application: uuid.NewV7(), Member: entities.SelfOwner})

	assert.Nil(t, output.Error)
	assert.NotNil(t, output.Data, "lista vazia é um array")
	assert.Empty(t, output.Data)
}

func whenGetSucceeds(t *testing.T) {
	r := newRig()
	id := uuid.NewV7()

	output := r.get().Perform(context.Background(), useCases.GetInput{Requester: r.requester(), Application: uuid.NewV7(), Member: entities.SelfOwner, Secret: id})

	assert.Nil(t, output.Error)
	assert.Equal(t, id, output.Data.ID)
	assert.Equal(t, r.owner, output.Data.MemberID)
}

func whenGetIsMissing(t *testing.T) {
	r := newRig()
	r.secrets.FindByOwnerFunc = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (entities.Secret, error) {
		return entities.Secret{}, sql.ErrNoRows
	}

	output := r.get().Perform(context.Background(), useCases.GetInput{Requester: r.requester(), Application: uuid.NewV7(), Member: entities.SelfOwner, Secret: uuid.NewV7()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeNotFound, output.Error.Type)
	}
}

func whenVersionsSucceed(t *testing.T) {
	r := newRig()
	versions := []entities.SecretVersion{{Version: 2}, {Version: 1}}
	r.versions.ListFunc = func(context.Context, uuid.UUID, uuid.UUID, int, int) ([]entities.SecretVersion, int, error) {
		return versions, len(versions), nil
	}

	output := r.listVersions().Perform(context.Background(), useCases.VersionsInput{Requester: r.requester(), Application: uuid.NewV7(), Member: entities.SelfOwner, Secret: uuid.NewV7()})

	assert.Nil(t, output.Error)
	assert.Equal(t, versions, output.Data)
	assert.NotEqual(t, []uuid.UUID{{}}, r.versions.Secrets, "lista as versões do segredo achado")
	assert.NotEmpty(t, r.versions.Applications, "e só da aplicação pedida")
}

func whenVersionsSecretIsMissing(t *testing.T) {
	r := newRig()
	r.secrets.FindByOwnerFunc = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (entities.Secret, error) {
		return entities.Secret{}, sql.ErrNoRows
	}
	r.versions.ListFunc = func(context.Context, uuid.UUID, uuid.UUID, int, int) ([]entities.SecretVersion, int, error) {
		t.Error("sem o segredo, as versões não são lidas")
		return nil, 0, nil
	}

	output := r.listVersions().Perform(context.Background(), useCases.VersionsInput{Requester: r.requester(), Application: uuid.NewV7(), Member: entities.SelfOwner, Secret: uuid.NewV7()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusNotFound, output.Error.Status)
	}
}

func whenReadIsDenied(t *testing.T) {
	r := newRig()
	r.guard.Deny(entities.ForbiddenError())
	r.secrets.ListFunc = func(context.Context, uuid.UUID, uuid.UUID, int, int) ([]entities.Secret, int, error) {
		t.Error("negado, o repositório não é chamado")
		return nil, 0, nil
	}

	list := r.list().Perform(context.Background(), useCases.ListInput{Requester: r.requester(), Application: uuid.NewV7(), Member: uuid.NewV7()})
	get := r.get().Perform(context.Background(), useCases.GetInput{Requester: r.requester(), Application: uuid.NewV7(), Member: uuid.NewV7(), Secret: uuid.NewV7()})
	versions := r.listVersions().Perform(context.Background(), useCases.VersionsInput{Requester: r.requester(), Application: uuid.NewV7(), Member: uuid.NewV7(), Secret: uuid.NewV7()})

	for _, denied := range []*entities.Error{list.Error, get.Error, versions.Error} {
		if assert.NotNil(t, denied) {
			assert.Equal(t, http.StatusForbidden, denied.Status)
		}
	}
}

func whenReadFailsInDb(t *testing.T) {
	r := newRig()
	boom := errors.New("segredo-interno")
	r.secrets.ListFunc = func(context.Context, uuid.UUID, uuid.UUID, int, int) ([]entities.Secret, int, error) {
		return nil, 0, boom
	}
	r.secrets.FindByOwnerFunc = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (entities.Secret, error) {
		return entities.Secret{}, boom
	}

	list := r.list().Perform(context.Background(), useCases.ListInput{Requester: r.requester(), Application: uuid.NewV7(), Member: entities.SelfOwner})
	get := r.get().Perform(context.Background(), useCases.GetInput{Requester: r.requester(), Application: uuid.NewV7(), Member: entities.SelfOwner, Secret: uuid.NewV7()})

	for _, failed := range []*entities.Error{list.Error, get.Error} {
		if assert.NotNil(t, failed) {
			assert.Equal(t, entities.ErrTypeInternal, failed.Type)
			assert.NotContains(t, failed.Detail, "segredo-interno")
		}
	}
}
