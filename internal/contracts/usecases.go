package contracts

import "context"

type IUseCase[i any, o any] interface {
	Perform(ctx context.Context, input i) o
}
