package grpcx_test

import (
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/grpcx"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestMessages(t *testing.T) {
	t.Run("Quando converte valores opcionais.", func(t *testing.T) { whenGRPCOptionalsAreConverted(t) })
	t.Run("Quando converte um membro.", func(t *testing.T) { whenGRPCMemberIsConverted(t) })
}

// Functions

func whenGRPCOptionalsAreConverted(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	id := uuid.NewV7()
	paginator := entities.NewPaginator(2, 10, 35)

	assert.True(t, now.Equal(grpcx.Time(now).AsTime()))
	assert.True(t, now.Equal(grpcx.OptionalTime(&now).AsTime()))
	assert.Nil(t, grpcx.OptionalTime(nil), "data ausente fica ausente, não vira 1970")
	assert.Equal(t, id.String(), grpcx.OptionalUUID(&id))
	assert.Empty(t, grpcx.OptionalUUID(nil))
	assert.Nil(t, grpcx.Page(nil))
	page := grpcx.Page(&paginator)
	assert.Equal(t, int32(2), page.GetPage())
	assert.Equal(t, int32(10), page.GetLimit())
	assert.Equal(t, int32(35), page.GetTotal())
	assert.True(t, page.GetHasNext())
	assert.True(t, page.GetHasPrev())
}

func whenGRPCMemberIsConverted(t *testing.T) {
	inviter := uuid.NewV7()
	updatedAt := time.Now().UTC().Truncate(time.Second)
	owner := entities.Member{ID: uuid.NewV7(), ApplicationID: uuid.NewV7(), UserID: uuid.NewV7(), UriID: "uri", Role: entities.RoleAdmin, Status: entities.MemberActive, CreatedAt: updatedAt}
	invited := owner
	invited.InvitedBy, invited.UpdatedAt = &inviter, &updatedAt

	first, second := grpcx.MemberMessage(owner), grpcx.MemberMessage(invited)

	assert.Equal(t, owner.ID.String(), first.GetId())
	assert.Equal(t, owner.ApplicationID.String(), first.GetApplicationId())
	assert.Equal(t, owner.UserID.String(), first.GetUserId())
	assert.Equal(t, "uri", first.GetUriId())
	assert.Equal(t, string(entities.RoleAdmin), first.GetRole())
	assert.Equal(t, string(entities.MemberActive), first.GetStatus())
	assert.Empty(t, first.GetInvitedBy())
	assert.Nil(t, first.GetUpdatedAt())
	assert.Equal(t, inviter.String(), second.GetInvitedBy())
	assert.True(t, updatedAt.Equal(second.GetUpdatedAt().AsTime()))
}
