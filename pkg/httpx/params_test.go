package httpx_test

import (
	"testing"

	"github.com/murkio-dev/vault/pkg/httpx"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestAssertNotNilParams(t *testing.T) {
	t.Run("Quando todos os parâmetros existem.", func(t *testing.T) { whenAllParamsAreSet(t) })
	t.Run("Quando algum parâmetro é nil.", func(t *testing.T) { whenSomeParamIsNil(t) })
}

// Functions

func whenAllParamsAreSet(t *testing.T) {
	err := httpx.AssertNotNilParams(map[string]any{
		"texto": "a", "numero": 0, "struct": struct{}{}, "ponteiro": &struct{}{},
		"mapa": map[string]int{}, "lista": []int{}, "funcao": func() {}, "canal": make(chan int),
	})

	assert.NoError(t, err, "valor vazio mas existente não é nil")
}

func whenSomeParamIsNil(t *testing.T) {
	var (
		pointer *struct{}
		mapping map[string]int
		list    []int
		fn      func()
		channel chan int
		face    error
	)

	for name, param := range map[string]any{"nil": nil, "ponteiro": pointer, "mapa": mapping, "lista": list, "funcao": fn, "canal": channel, "interface": face} {
		err := httpx.AssertNotNilParams(map[string]any{name: param})

		if assert.Error(t, err, name) {
			assert.Contains(t, err.Error(), name, "o erro diz qual parâmetro faltou")
		}
	}

	err := httpx.AssertNotNilParams(map[string]any{"a": nil, "b": pointer, "c": "ok"})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "'a'")
		assert.Contains(t, err.Error(), "'b'")
		assert.NotContains(t, err.Error(), "'c'", "só os que faltam entram no erro")
	}
}
