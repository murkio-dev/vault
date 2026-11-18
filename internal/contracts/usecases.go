package contracts

import (
	"context"

	"github.com/murkio-dev/vault/internal/entities"
)

type IUseCase[i any, o any] interface {
	Perform(ctx context.Context, input i) o
}

type IAccessGuard = IUseCase[entities.AuthorizeInput, entities.AuthorizeOutput]
