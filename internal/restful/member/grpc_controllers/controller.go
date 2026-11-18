package grpc_controllers

import (
	"context"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/member/useCases"
	"github.com/murkio-dev/vault/pkg/grpcx"
	"github.com/murkio-dev/vault/pkg/httpx"
	vaultv1 "github.com/murkio-dev/vault/pkg/rpc/vaultv1"
	"google.golang.org/grpc"
)

type RegisterInput struct {
	Registrar     grpc.ServiceRegistrar
	ListUseCase   contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
	GetMeUseCase  contracts.IUseCase[useCases.GetMeInput, useCases.GetMeOutput]
	UpdateUseCase contracts.IUseCase[useCases.UpdateInput, useCases.UpdateOutput]
	RemoveUseCase contracts.IUseCase[useCases.RemoveInput, useCases.RemoveOutput]
}

func RegisterControllers(input RegisterInput) error {
	err := httpx.AssertNotNilParams(map[string]any{
		"input.Registrar":     input.Registrar,
		"input.ListUseCase":   input.ListUseCase,
		"input.GetMeUseCase":  input.GetMeUseCase,
		"input.UpdateUseCase": input.UpdateUseCase,
		"input.RemoveUseCase": input.RemoveUseCase,
	})
	if err != nil {
		return err
	}

	vaultv1.RegisterMemberServiceServer(input.Registrar, controller{
		listUseCase:   input.ListUseCase,
		getMeUseCase:  input.GetMeUseCase,
		updateUseCase: input.UpdateUseCase,
		removeUseCase: input.RemoveUseCase,
	})

	return nil
}

type controller struct {
	vaultv1.UnimplementedMemberServiceServer

	listUseCase   contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
	getMeUseCase  contracts.IUseCase[useCases.GetMeInput, useCases.GetMeOutput]
	updateUseCase contracts.IUseCase[useCases.UpdateInput, useCases.UpdateOutput]
	removeUseCase contracts.IUseCase[useCases.RemoveInput, useCases.RemoveOutput]
}

func (c controller) ListMembers(ctx context.Context, req *vaultv1.ListMembersRequest) (*vaultv1.ListMembersResponse, error) {

	application, e := grpcx.UUID(req.GetApplicationId(), "application_id")
	if e != nil {
		return nil, grpcx.Error(*e)
	}
	page, limit, e := grpcx.Pagination(req.GetPage(), req.GetLimit())
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.listUseCase.Perform(ctx, useCases.ListInput{Requester: grpcx.Requester(ctx), Application: application, Page: page, Limit: limit})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	members := make([]*vaultv1.Member, len(output.Data))
	for i, member := range output.Data {
		members[i] = grpcx.MemberMessage(member)
	}

	return &vaultv1.ListMembersResponse{Members: members, Page: grpcx.Page(output.Page)}, nil
}

func (c controller) GetMe(ctx context.Context, req *vaultv1.GetMeRequest) (*vaultv1.Member, error) {

	application, e := grpcx.UUID(req.GetApplicationId(), "application_id")
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.getMeUseCase.Perform(ctx, useCases.GetMeInput{Requester: grpcx.Requester(ctx), Application: application})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return grpcx.MemberMessage(output.Data), nil
}

func (c controller) UpdateMember(ctx context.Context, req *vaultv1.UpdateMemberRequest) (*vaultv1.Member, error) {

	application, e := grpcx.UUID(req.GetApplicationId(), "application_id")
	if e != nil {
		return nil, grpcx.Error(*e)
	}
	member, e := grpcx.UUID(req.GetMemberId(), "member_id")
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	input := useCases.UpdateInput{Requester: grpcx.Requester(ctx), Application: application, Member: member}
	if req.Role != nil {
		role := entities.Role(req.GetRole())
		input.Role = &role
	}
	if req.Status != nil {
		status := entities.MemberStatus(req.GetStatus())
		input.Status = &status
	}

	output := c.updateUseCase.Perform(ctx, input)
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return grpcx.MemberMessage(output.Data), nil
}

func (c controller) RemoveMember(ctx context.Context, req *vaultv1.RemoveMemberRequest) (*vaultv1.RemoveMemberResponse, error) {

	application, e := grpcx.UUID(req.GetApplicationId(), "application_id")
	if e != nil {
		return nil, grpcx.Error(*e)
	}
	member, e := grpcx.UUID(req.GetMemberId(), "member_id")
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.removeUseCase.Perform(ctx, useCases.RemoveInput{Requester: grpcx.Requester(ctx), Application: application, Member: member})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return &vaultv1.RemoveMemberResponse{}, nil
}
