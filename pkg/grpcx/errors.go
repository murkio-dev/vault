package grpcx

import (
	"net/http"

	"github.com/murkio-dev/vault/internal/entities"
	vaultv1 "github.com/murkio-dev/vault/pkg/rpc/vaultv1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Error converte o erro de um use case em status gRPC pela classe do status
// HTTP dele, o equivalente do RespondError da entrega HTTP. Existe para os
// grpc_controllers e o interceptor de autenticação responderem do mesmo jeito.
// Recebe o entities.Error que o use case devolveu. O Detail é público (já sai
// na API HTTP) e vira a mensagem; sem Detail, usa o Title.
// Devolve sempre um erro de status: 400 e 422 viram InvalidArgument, 401
// Unauthenticated, 403 PermissionDenied, 404 NotFound, 409 AlreadyExists, 429
// ResourceExhausted, e qualquer outro status vira Internal.
// Exemplo: NotFoundError("Resource") vira codes.NotFound, e um erro 500 com
// Detail "disk full" vira codes.Internal com a mensagem fixa "internal error".
// O texto fixo no Internal é de propósito: a mensagem de banco nunca sai pelo canal.
func Error(e entities.Error) error {

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

// ItemErrors converte os erros de um lote nos ItemError da resposta. Existe
// porque num sucesso parcial os erros viajam dentro da resposta, e não como
// status gRPC, então o corte do erro interno do Error não os alcança.
// Recebe os erros do use case, na ordem do lote.
// Devolve um ItemError por erro, com status, type, title e detail. Lista vazia
// devolve nil.
// Exemplo: um NotFoundError vira {status: 404, type: "...#not-found", ...}; um
// erro 500 sai só com status, type e title, sem detail.
// O detail do erro interno é cortado de propósito: nada que venha do banco sai
// pelo canal, mesmo que o use case tenha preenchido o campo.
func ItemErrors(errs []entities.Error) []*vaultv1.ItemError {
	if len(errs) == 0 {
		return nil
	}

	items := make([]*vaultv1.ItemError, len(errs))
	for i, e := range errs {
		item := &vaultv1.ItemError{Status: int32(e.Status), Type: e.Type, Title: e.Title}
		if e.Status < http.StatusInternalServerError {
			item.Detail = e.Detail
		}
		items[i] = item
	}
	return items
}
