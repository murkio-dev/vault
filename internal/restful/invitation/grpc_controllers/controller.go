package grpc_controllers

import (
	"context"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/invitation/useCases"
	"github.com/murkio-dev/vault/pkg/grpcx"
	"github.com/murkio-dev/vault/pkg/httpx"
	vaultv1 "github.com/murkio-dev/vault/pkg/rpc/vaultv1"
	"google.golang.org/grpc"
)

type RegisterInput struct {
	Registrar     grpc.ServiceRegistrar
	CreateUseCase contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]
	ListUseCase   contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
	RevokeUseCase contracts.IUseCase[useCases.RevokeInput, useCases.RevokeOutput]
	AcceptUseCase contracts.IUseCase[useCases.AcceptInput, useCases.AcceptOutput]
}

func RegisterControllers(input RegisterInput) error {
	err := httpx.AssertNotNilParams(map[string]any{
		"input.Registrar":     input.Registrar,
		"input.CreateUseCase": input.CreateUseCase,
		"input.ListUseCase":   input.ListUseCase,
		"input.RevokeUseCase": input.RevokeUseCase,
		"input.AcceptUseCase": input.AcceptUseCase,
	})
	if err != nil {
		return err
	}

	vaultv1.RegisterInvitationServiceServer(input.Registrar, controller{
		createUseCase: input.CreateUseCase,
		listUseCase:   input.ListUseCase,
		revokeUseCase: input.RevokeUseCase,
		acceptUseCase: input.AcceptUseCase,
	})

	return nil
}

type controller struct {
	vaultv1.UnimplementedInvitationServiceServer

	createUseCase contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]
	listUseCase   contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
	revokeUseCase contracts.IUseCase[useCases.RevokeInput, useCases.RevokeOutput]
	acceptUseCase contracts.IUseCase[useCases.AcceptInput, useCases.AcceptOutput]
}

func (c controller) CreateInvitation(ctx context.Context, req *vaultv1.CreateInvitationRequest) (*vaultv1.CreateInvitationResponse, error) {

	application, e := grpcx.UUID(req.GetApplicationId(), "application_id")
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.createUseCase.Perform(ctx, useCases.CreateInput{
		Requester: grpcx.Requester(ctx), Application: application, Email: req.GetEmail(), Role: entities.Role(req.GetRole()),
	})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return &vaultv1.CreateInvitationResponse{Invitation: toMessage(output.Data.Invitation, ""), Token: output.Data.Token}, nil
}

func (c controller) ListInvitations(ctx context.Context, req *vaultv1.ListInvitationsRequest) (*vaultv1.ListInvitationsResponse, error) {

	application, e := grpcx.UUID(req.GetApplicationId(), "application_id")
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.listUseCase.Perform(ctx, useCases.ListInput{Requester: grpcx.Requester(ctx), Application: application})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	invitations := make([]*vaultv1.Invitation, len(output.Data))
	for i, view := range output.Data {
		invitations[i] = toMessage(view.Invitation, view.Status)
	}

	return &vaultv1.ListInvitationsResponse{Invitations: invitations}, nil
}

func (c controller) RevokeInvitation(ctx context.Context, req *vaultv1.RevokeInvitationRequest) (*vaultv1.RevokeInvitationResponse, error) {

	application, e := grpcx.UUID(req.GetApplicationId(), "application_id")
	if e != nil {
		return nil, grpcx.Error(*e)
	}
	invitation, e := grpcx.UUID(req.GetInvitationId(), "invitation_id")
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.revokeUseCase.Perform(ctx, useCases.RevokeInput{Requester: grpcx.Requester(ctx), Application: application, Invitation: invitation})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return &vaultv1.RevokeInvitationResponse{}, nil
}

func (c controller) AcceptInvitation(ctx context.Context, req *vaultv1.AcceptInvitationRequest) (*vaultv1.Member, error) {

	output := c.acceptUseCase.Perform(ctx, useCases.AcceptInput{Requester: grpcx.Requester(ctx), Token: req.GetToken()})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return grpcx.MemberMessage(output.Data), nil
}

func toMessage(invitation entities.Invitation, status entities.InvitationStatus) *vaultv1.Invitation {
	return &vaultv1.Invitation{
		Id:            invitation.ID.String(),
		ApplicationId: invitation.ApplicationID.String(),
		Email:         invitation.Email,
		Role:          string(invitation.Role),
		ExpiresAt:     grpcx.Time(invitation.ExpiresAt),
		CreatedBy:     invitation.CreatedBy.String(),
		AcceptedBy:    grpcx.OptionalUUID(invitation.AcceptedBy),
		AcceptedAt:    grpcx.OptionalTime(invitation.AcceptedAt),
		RevokedAt:     grpcx.OptionalTime(invitation.RevokedAt),
		CreatedAt:     grpcx.Time(invitation.CreatedAt),
		Status:        string(status),
	}
}
