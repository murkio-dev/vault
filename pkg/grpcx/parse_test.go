package grpcx_test

import (
	"fmt"
	"net/http"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/grpcx"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestParse(t *testing.T) {
	t.Run("Quando lê um uuid e um membro.", func(t *testing.T) { whenGRPCIDsAreParsed(t) })
	t.Run("Quando lê a paginação.", func(t *testing.T) { whenGRPCPaginationIsParsed(t) })
}

// Functions

func whenGRPCIDsAreParsed(t *testing.T) {
	id := uuid.NewV7()

	parsed, e := grpcx.UUID(id.String(), "secret_id")
	me, meErr := grpcx.MemberID("me")
	member, memberErr := grpcx.MemberID(id.String())

	assert.Nil(t, e)
	assert.Equal(t, id, parsed)
	assert.Nil(t, meErr)
	assert.Equal(t, entities.SelfOwner, me)
	assert.Nil(t, memberErr)
	assert.Equal(t, id, member)

	for _, raw := range []string{"", "nao-e-uuid", uuid.UUID{}.String(), "ME", "' OR 1=1 --"} {
		_, uuidErr := grpcx.UUID(raw, "secret_id")
		_, memberErr := grpcx.MemberID(raw)

		for _, invalid := range []*entities.Error{uuidErr, memberErr} {
			if assert.NotNil(t, invalid, raw) {
				assert.Equal(t, entities.ErrTypeValidation, invalid.Type, raw)
				assert.Equal(t, http.StatusBadRequest, invalid.Status, raw)
				assert.NotContains(t, fmt.Sprint(invalid.Detail, invalid.InvalidParams), "OR 1=1", "o erro não repete o valor recebido")
			}
		}
	}
	_, me2 := grpcx.UUID("me", "token_id")
	assert.NotNil(t, me2, "\"me\" só vale onde o campo é o membro")
}

func whenGRPCPaginationIsParsed(t *testing.T) {
	defaultPage, defaultLimit, defaultErr := grpcx.Pagination(0, 0)
	page, limit, e := grpcx.Pagination(3, 20)
	_, _, negativePage := grpcx.Pagination(-1, 10)
	_, _, negativeLimit := grpcx.Pagination(1, -10)

	assert.Nil(t, defaultErr)
	assert.Equal(t, 1, defaultPage, "zero é o campo ausente e usa a primeira página")
	assert.Equal(t, entities.MaxPageSize, defaultLimit)
	assert.Nil(t, e)
	assert.Equal(t, 3, page)
	assert.Equal(t, 20, limit)
	for _, invalid := range []*entities.Error{negativePage, negativeLimit} {
		if assert.NotNil(t, invalid) {
			assert.Equal(t, entities.ErrTypeValidation, invalid.Type)
		}
	}
}
