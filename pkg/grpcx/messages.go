package grpcx

import (
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	vaultv1 "github.com/murkio-dev/vault/pkg/rpc/vaultv1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func Page(paginator *entities.Paginator) *vaultv1.Page {
	if paginator == nil {
		return nil
	}
	return &vaultv1.Page{
		Page:    int32(paginator.Page),
		Limit:   int32(paginator.Limit),
		Total:   int32(paginator.Total),
		HasNext: paginator.HasNext,
		HasPrev: paginator.HasPrev,
	}
}

func Time(value time.Time) *timestamppb.Timestamp {
	return timestamppb.New(value)
}

func OptionalTime(value *time.Time) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}
	return timestamppb.New(*value)
}

func OptionalUUID(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

// MemberMessage converte um membro na mensagem do contrato gRPC. Fica aqui,
// e não no controller de membros, porque o convite aceito também devolve um
// membro (invitation/grpc_controllers) e um controller não importa o outro.
// Recebe o membro como o use case devolveu.
// Devolve a mensagem com os ids em texto; invited_by e updated_at ficam vazios
// quando o membro não tem esses dados.
// Exemplo: o criador da aplicação sai com invited_by "" e updated_at ausente.
// Nenhum campo do membro é segredo, então tudo o que a API HTTP mostra sai aqui.
func MemberMessage(member entities.Member) *vaultv1.Member {
	return &vaultv1.Member{
		Id:            member.ID.String(),
		ApplicationId: member.ApplicationID.String(),
		UserId:        member.UserID.String(),
		UriId:         member.UriID,
		Role:          string(member.Role),
		Status:        string(member.Status),
		InvitedBy:     OptionalUUID(member.InvitedBy),
		CreatedAt:     Time(member.CreatedAt),
		UpdatedAt:     OptionalTime(member.UpdatedAt),
	}
}
