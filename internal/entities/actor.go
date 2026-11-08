package entities

import (
	"context"
	"uuid"
)

type ActorKind string

const (
	ActorUser  ActorKind = "user"
	ActorToken ActorKind = "token"
)

type Actor struct {
	Kind          ActorKind
	UserID        uuid.UUID
	TokenID       uuid.UUID
	MemberID      uuid.UUID
	ApplicationID uuid.UUID
	PlatformAdmin bool
	SessionID     string
}

func (a Actor) Authenticated() bool {
	return (a.Kind == ActorUser || a.Kind == ActorToken) && a.UserID != (uuid.UUID{})
}

func (a Actor) AuditType() string {
	if a.Kind == ActorToken {
		return "token"
	}
	return "user"
}

type actorKey struct{}

func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, actor)
}

func ActorFrom(ctx context.Context) Actor {
	actor, _ := ctx.Value(actorKey{}).(Actor)
	return actor
}
