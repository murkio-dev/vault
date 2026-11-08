// Package mocks tem o fake único do contracts.IUseCase, que os testes dos
// controllers de todos os services usam. Os fakes de repository são concretos e
// ficam em mocks/ de cada service.
package mocks

import (
	"context"
	"sync"

	"github.com/murkio-dev/vault/internal/contracts"
)

// Mocks

// UseCase é o fake de contracts.IUseCase para os testes dos controllers: guarda
// cada Input recebido e devolve o Output de PerformFunc (ou o valor zero, que é
// um sucesso sem dados). É seguro para uso concorrente.
type UseCase[I, O any] struct {
	PerformFunc func(ctx context.Context, input I) O

	mu     sync.Mutex
	Inputs []I
}

func (m *UseCase[I, O]) Perform(ctx context.Context, input I) O {
	m.mu.Lock()
	m.Inputs = append(m.Inputs, input)
	m.mu.Unlock()
	if m.PerformFunc != nil {
		return m.PerformFunc(ctx, input)
	}
	var zero O
	return zero
}

var _ contracts.IUseCase[int, int] = (*UseCase[int, int])(nil)
