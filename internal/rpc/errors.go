package rpc

import (
	"net/http"

	"github.com/murkio-dev/vault/internal/entities"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// toStatus converte o erro de um use case em status gRPC pela classe do status
// HTTP dele. O Detail do erro é público (já sai na API HTTP), e o erro interno
// (5xx) vira Internal com texto fixo, para nunca vazar mensagem de banco pelo
// canal. Exemplo: NotFoundError (404) vira NotFound, UnauthorizedError (401)
// vira Unauthenticated e ValidationError (400) vira InvalidArgument.
func toStatus(e entities.Error) error {

	message := e.Title
	if e.Detail != "" {
		message = e.Detail
	}

	switch e.Status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return status.Error(codes.InvalidArgument, message)
	case http.StatusUnauthorized:
		return status.Error(codes.Unauthenticated, message)
	case http.StatusForbidden:
		return status.Error(codes.PermissionDenied, message)
	case http.StatusNotFound:
		return status.Error(codes.NotFound, message)
	case http.StatusConflict:
		return status.Error(codes.AlreadyExists, message)
	case http.StatusTooManyRequests:
		return status.Error(codes.ResourceExhausted, message)
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
