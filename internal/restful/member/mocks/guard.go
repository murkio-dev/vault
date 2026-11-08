package mocks

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
)

// Mocks

type Guard struct {
	PerformFunc func(ctx context.Context, input entities.AuthorizeInput) entities.AuthorizeOutput
	Member      entities.Member

	Calls  int
	Inputs []entities.AuthorizeInput
}

func (m *Guard) Perform(ctx context.Context, input entities.AuthorizeInput) entities.AuthorizeOutput {
	m.Calls++
	m.Inputs = append(m.Inputs, input)

	if m.PerformFunc != nil {
		return m.PerformFunc(ctx, input)
	}

	member := m.Member
	if member.ID == (uuid.UUID{}) {
		member = NewMember(entities.RoleAdmin)
	}

	owner := input.Owner
	if owner == entities.SelfOwner {
		owner = member.ID
	}

	return entities.AuthorizeOutput{Member: member, Owner: owner}
}

func (m *Guard) Deny(denied entities.Error) *Guard {
	m.PerformFunc = func(context.Context, entities.AuthorizeInput) entities.AuthorizeOutput {
		return entities.AuthorizeOutput{Error: &denied}
	}
	return m
}

func (m *Guard) Actions() []entities.Action {
	actions := make([]entities.Action, len(m.Inputs))
	for i, input := range m.Inputs {
		actions[i] = input.Action
	}
	return actions
}

var _ contracts.IAccessGuard = (*Guard)(nil)

// Helpers

func NewMember(role entities.Role) entities.Member {
	return entities.NewMember(uuid.NewV7(), uuid.NewV7(), role, nil)
}

func NewUserRequester() entities.ApiHeader {
	return entities.ApiHeader{
		ClientIP:  "203.0.113.7",
		UserAgent: "go-test",
		RequestID: uuid.New().String(),
		Actor:     entities.Actor{Kind: entities.ActorUser, UserID: uuid.NewV7()},
	}
}

func NewTokenRequester(member entities.Member) entities.ApiHeader {
	return entities.ApiHeader{
		ClientIP:  "203.0.113.7",
		UserAgent: "go-sdk",
		RequestID: uuid.New().String(),
		Actor: entities.Actor{Kind: entities.ActorToken, UserID: member.UserID, TokenID: uuid.NewV7(),
			MemberID: member.ID, ApplicationID: member.ApplicationID},
	}
}
