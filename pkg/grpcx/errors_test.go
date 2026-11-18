package grpcx_test

import (
	"net/http"
	"testing"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/grpcx"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Main test

func TestError(t *testing.T) {
	t.Run("Quando o erro do use case vira status.", func(t *testing.T) { whenGRPCErrorMapsStatus(t) })
	t.Run("Quando o erro é interno.", func(t *testing.T) { whenGRPCErrorIsInternal(t) })
	t.Run("Quando converte os erros de um lote.", func(t *testing.T) { whenGRPCItemErrorsAreConverted(t) })
}

// Functions

func whenGRPCErrorMapsStatus(t *testing.T) {
	cases := map[int]codes.Code{
		http.StatusBadRequest:          codes.InvalidArgument,
		http.StatusUnprocessableEntity: codes.InvalidArgument,
		http.StatusUnauthorized:        codes.Unauthenticated,
		http.StatusForbidden:           codes.PermissionDenied,
		http.StatusNotFound:            codes.NotFound,
		http.StatusConflict:            codes.AlreadyExists,
		http.StatusTooManyRequests:     codes.ResourceExhausted,
	}
	for httpStatus, code := range cases {
		withDetail := grpcx.Error(entities.Error{Status: httpStatus, Title: "titulo", Detail: "detalhe"})
		withoutDetail := grpcx.Error(entities.Error{Status: httpStatus, Title: "titulo"})

		assert.Equal(t, code, status.Code(withDetail), httpStatus)
		assert.Equal(t, "detalhe", status.Convert(withDetail).Message(), httpStatus)
		assert.Equal(t, "titulo", status.Convert(withoutDetail).Message(), "sem Detail, sai o Title")
	}
}

func whenGRPCErrorIsInternal(t *testing.T) {
	for _, httpStatus := range []int{http.StatusInternalServerError, http.StatusBadGateway, 0} {
		err := grpcx.Error(entities.Error{Status: httpStatus, Title: "titulo-interno", Detail: "segredo-interno"})

		assert.Equal(t, codes.Internal, status.Code(err), httpStatus)
		assert.Equal(t, "internal error", status.Convert(err).Message(), "o texto é fixo: nada do erro original sai")
	}
}

func whenGRPCItemErrorsAreConverted(t *testing.T) {
	notFound := entities.NotFoundError("Resource")
	internal := entities.Error{Status: http.StatusInternalServerError, Type: entities.ErrTypeInternal, Title: "Internal Server Error", Detail: "segredo-interno"}

	items := grpcx.ItemErrors([]entities.Error{notFound, internal})

	assert.Nil(t, grpcx.ItemErrors(nil))
	assert.Nil(t, grpcx.ItemErrors([]entities.Error{}))
	if assert.Len(t, items, 2, "um por erro, na ordem do lote") {
		assert.Equal(t, int32(http.StatusNotFound), items[0].GetStatus())
		assert.Equal(t, entities.ErrTypeNotFound, items[0].GetType())
		assert.Equal(t, notFound.Title, items[0].GetTitle())
		assert.Equal(t, notFound.Detail, items[0].GetDetail())
		assert.Equal(t, int32(http.StatusInternalServerError), items[1].GetStatus())
		assert.Empty(t, items[1].GetDetail(), "o detalhe do erro interno é cortado")
	}
}
