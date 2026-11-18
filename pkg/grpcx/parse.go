package grpcx

import (
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
)

func UUID(raw, field string) (uuid.UUID, *entities.Error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == (uuid.UUID{}) {
		invalid := entities.ValidationError("The field "+field+" is invalid.",
			entities.InvalidParam{Field: field, Reason: "must be a valid, non-empty uuid"})
		return uuid.UUID{}, &invalid
	}
	return id, nil
}

// MemberID lê o member_id de um pedido gRPC: um UUID, ou a palavra "me", que
// vira entities.SelfOwner. É o equivalente do PathMember da entrega HTTP, para
// "meus segredos" e "os segredos do membro X" passarem pela mesma regra de
// permissão nas duas entregas.
// Recebe o valor cru do campo. Qualquer texto que não seja "me" nem um UUID
// não-zero é erro de validação, e o erro não repete o valor recebido.
// Devolve o id do membro (ou o marcador SelfOwner) e nil, ou o uuid zero e o erro.
// Exemplo: "me" devolve entities.SelfOwner; "abc" devolve erro no campo member_id.
// O marcador só é trocado pelo membro do ator no use case de autorização.
func MemberID(raw string) (uuid.UUID, *entities.Error) {
	if raw == "me" {
		return entities.SelfOwner, nil
	}
	return UUID(raw, "member_id")
}

// Pagination lê page e limit de um pedido gRPC, o equivalente do Pagination
// da entrega HTTP. Existe porque no proto3 o campo ausente chega como zero, e
// zero tem de significar "use o padrão", não "página zero".
// Recebe os dois campos do pedido. Zero usa o padrão (página 1 e
// entities.MaxPageSize); valor negativo é erro de validação.
// Devolve page e limit prontos para o use case, ou zeros e o erro.
// Exemplo: (0, 0) devolve (1, 150); (2, 10) devolve (2, 10); (-1, 0) devolve erro.
// O teto do limit não é aplicado aqui: é regra do use case, igual no HTTP.
func Pagination(page, limit int32) (int, int, *entities.Error) {
	if page < 0 {
		invalid := entities.ValidationError("The field page is invalid.", entities.InvalidParam{Field: "page", Reason: "must be a positive integer"})
		return 0, 0, &invalid
	}
	if limit < 0 {
		invalid := entities.ValidationError("The field limit is invalid.", entities.InvalidParam{Field: "limit", Reason: "must be a positive integer"})
		return 0, 0, &invalid
	}

	if page == 0 {
		page = 1
	}
	if limit == 0 {
		limit = entities.MaxPageSize
	}

	return int(page), int(limit), nil
}
