package grpc_controllers

import (
	"context"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/audit/services"
	"github.com/murkio-dev/vault/internal/restful/audit/useCases"
	"github.com/murkio-dev/vault/pkg/grpcx"
	"github.com/murkio-dev/vault/pkg/httpx"
	vaultv1 "github.com/murkio-dev/vault/pkg/rpc/vaultv1"
	"google.golang.org/grpc"
)

type RegisterInput struct {
	Registrar   grpc.ServiceRegistrar
	ListUseCase contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
}

func RegisterControllers(input RegisterInput) error {
	err := httpx.AssertNotNilParams(map[string]any{
		"input.Registrar":   input.Registrar,
		"input.ListUseCase": input.ListUseCase,
	})
	if err != nil {
		return err
	}

	vaultv1.RegisterAuditServiceServer(input.Registrar, controller{listUseCase: input.ListUseCase})

	return nil
}

type controller struct {
	vaultv1.UnimplementedAuditServiceServer

	listUseCase contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
}

func (c controller) ListAuditLogs(ctx context.Context, req *vaultv1.ListAuditLogsRequest) (*vaultv1.ListAuditLogsResponse, error) {

	application, filter, e := services.ParseGRPCList(req)
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.listUseCase.Perform(ctx, useCases.ListInput{Requester: grpcx.Requester(ctx), Application: application, Filter: filter})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	response := &vaultv1.ListAuditLogsResponse{Logs: make([]*vaultv1.AuditLog, len(output.Data))}
	for i, log := range output.Data {
		response.Logs[i] = toMessage(log)
	}
	if output.Page != nil && output.Page.NextCursor != nil {
		response.NextCursor = *output.Page.NextCursor
	}

	return response, nil
}

func toMessage(log entities.AuditLog) *vaultv1.AuditLog {
	return &vaultv1.AuditLog{
		Id:            log.ID.String(),
		ApplicationId: grpcx.OptionalUUID(log.ApplicationID),
		ActorType:     log.ActorType,
		ActorId:       grpcx.OptionalUUID(log.ActorID),
		TokenId:       grpcx.OptionalUUID(log.TokenID),
		Action:        log.Action,
		Outcome:       string(log.Outcome),
		TargetType:    log.TargetType,
		TargetId:      log.TargetID,
		Ip:            log.IP,
		UserAgent:     log.UserAgent,
		RequestId:     log.RequestID,
		Metadata:      string(log.Metadata),
		CreatedAt:     grpcx.Time(log.CreatedAt),
	}
}
