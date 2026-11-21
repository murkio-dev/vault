package grpc_controllers

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/apikeys/useCases"
	"github.com/murkio-dev/vault/pkg/grpcx"
	"github.com/murkio-dev/vault/pkg/httpx"
	vaultv1 "github.com/murkio-dev/vault/pkg/rpc/vaultv1"
	"google.golang.org/grpc"
)

type RegisterInput struct {
	Registrar      grpc.ServiceRegistrar
	CreateUseCase  contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]
	ListUseCase    contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
	GetUseCase     contracts.IUseCase[useCases.GetInput, useCases.GetOutput]
	UpdateUseCase  contracts.IUseCase[useCases.UpdateInput, useCases.UpdateOutput]
	UpgradeUseCase contracts.IUseCase[useCases.UpdateInput, useCases.UpdateOutput]
	DeleteUseCase  contracts.IUseCase[useCases.DeleteInput, useCases.DeleteOutput]
	RotateUseCase  contracts.IUseCase[useCases.RotateInput, useCases.RotateOutput]
}

func RegisterControllers(input RegisterInput) error {
	err := httpx.AssertNotNilParams(map[string]any{
		"input.Registrar":      input.Registrar,
		"input.CreateUseCase":  input.CreateUseCase,
		"input.ListUseCase":    input.ListUseCase,
		"input.GetUseCase":     input.GetUseCase,
		"input.UpdateUseCase":  input.UpdateUseCase,
		"input.UpgradeUseCase": input.UpgradeUseCase,
		"input.DeleteUseCase":  input.DeleteUseCase,
		"input.RotateUseCase":  input.RotateUseCase,
	})
	if err != nil {
		return err
	}

	vaultv1.RegisterApiKeyServiceServer(input.Registrar, controller{
		createUseCase:  input.CreateUseCase,
		listUseCase:    input.ListUseCase,
		getUseCase:     input.GetUseCase,
		updateUseCase:  input.UpdateUseCase,
		upgradeUseCase: input.UpgradeUseCase,
		deleteUseCase:  input.DeleteUseCase,
		rotateUseCase:  input.RotateUseCase,
	})

	return nil
}

type controller struct {
	vaultv1.UnimplementedApiKeyServiceServer

	createUseCase  contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]
	listUseCase    contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
	getUseCase     contracts.IUseCase[useCases.GetInput, useCases.GetOutput]
	updateUseCase  contracts.IUseCase[useCases.UpdateInput, useCases.UpdateOutput]
	upgradeUseCase contracts.IUseCase[useCases.UpdateInput, useCases.UpdateOutput]
	deleteUseCase  contracts.IUseCase[useCases.DeleteInput, useCases.DeleteOutput]
	rotateUseCase  contracts.IUseCase[useCases.RotateInput, useCases.RotateOutput]
}

func (c controller) CreateApiKey(ctx context.Context, req *vaultv1.CreateApiKeyRequest) (*vaultv1.ApiKeyCreated, error) {

	application, member, e := parseOwner(req.GetApplicationId(), req.GetMemberId())
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	input := useCases.CreateInput{Requester: grpcx.Requester(ctx), Application: application, Member: member, Name: req.GetName(), Type: entities.KeyType(req.GetType())}
	if req.GetExpiresAt() != nil {
		expiresAt := req.GetExpiresAt().AsTime()
		input.ExpiresAt = &expiresAt
	}

	output := c.createUseCase.Perform(ctx, input)
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return createdMessage(output.Data), nil
}

func (c controller) ListApiKeys(ctx context.Context, req *vaultv1.ListApiKeysRequest) (*vaultv1.ListApiKeysResponse, error) {

	application, member, e := parseOwner(req.GetApplicationId(), req.GetMemberId())
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.listUseCase.Perform(ctx, useCases.ListInput{Requester: grpcx.Requester(ctx), Application: application, Member: member})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	keys := make([]*vaultv1.ApiKey, len(output.Data))
	for i, key := range output.Data {
		keys[i] = toMessage(key)
	}

	return &vaultv1.ListApiKeysResponse{ApiKeys: keys}, nil
}

func (c controller) GetApiKey(ctx context.Context, req *vaultv1.ApiKeyRequest) (*vaultv1.ApiKey, error) {

	application, member, id, e := parseByID(req.GetApplicationId(), req.GetMemberId(), req.GetId())
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.getUseCase.Perform(ctx, useCases.GetInput{Requester: grpcx.Requester(ctx), Application: application, Member: member, ID: id})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return toMessage(output.Data), nil
}

func (c controller) UpdateApiKey(ctx context.Context, req *vaultv1.ChangeApiKeyRequest) (*vaultv1.ApiKey, error) {
	return change(ctx, c.updateUseCase, req)
}

func (c controller) UpgradeApiKey(ctx context.Context, req *vaultv1.ChangeApiKeyRequest) (*vaultv1.ApiKey, error) {
	return change(ctx, c.upgradeUseCase, req)
}

func (c controller) DeleteApiKey(ctx context.Context, req *vaultv1.ApiKeyRequest) (*vaultv1.Empty, error) {

	application, member, id, e := parseByID(req.GetApplicationId(), req.GetMemberId(), req.GetId())
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.deleteUseCase.Perform(ctx, useCases.DeleteInput{Requester: grpcx.Requester(ctx), Application: application, Member: member, ID: id})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return &vaultv1.Empty{}, nil
}

func (c controller) RotateApiKey(ctx context.Context, req *vaultv1.ApiKeyRequest) (*vaultv1.ApiKeyCreated, error) {

	application, member, id, e := parseByID(req.GetApplicationId(), req.GetMemberId(), req.GetId())
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.rotateUseCase.Perform(ctx, useCases.RotateInput{Requester: grpcx.Requester(ctx), Application: application, Member: member, ID: id})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return createdMessage(output.Data), nil
}

func change(ctx context.Context, usecase contracts.IUseCase[useCases.UpdateInput, useCases.UpdateOutput], req *vaultv1.ChangeApiKeyRequest) (*vaultv1.ApiKey, error) {

	application, member, id, e := parseByID(req.GetApplicationId(), req.GetMemberId(), req.GetId())
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := usecase.Perform(ctx, useCases.UpdateInput{Requester: grpcx.Requester(ctx), Application: application, Member: member, ID: id, Name: req.GetName()})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return toMessage(output.Data), nil
}

func parseOwner(rawApplication, rawMember string) (application, member uuid.UUID, e *entities.Error) {
	if application, e = grpcx.UUID(rawApplication, "application_id"); e != nil {
		return
	}
	member, e = grpcx.MemberID(rawMember)
	return
}

func parseByID(rawApplication, rawMember, rawID string) (application, member, id uuid.UUID, e *entities.Error) {
	if application, member, e = parseOwner(rawApplication, rawMember); e != nil {
		return
	}
	id, e = grpcx.UUID(rawID, "id")
	return
}

func createdMessage(key entities.ApiKeyCreated) *vaultv1.ApiKeyCreated {
	return &vaultv1.ApiKeyCreated{ApiKey: toMessage(key.ApiKey), Key: key.Key, Uri: key.URI}
}

func toMessage(key entities.ApiKey) *vaultv1.ApiKey {
	return &vaultv1.ApiKey{
		Id:            key.ID.String(),
		ApplicationId: key.ApplicationID.String(),
		MemberId:      key.MemberID.String(),
		UriId:         key.UriID,
		Name:          key.Name,
		Type:          string(key.Type),
		Prefix:        key.Prefix,
		LastUsedAt:    grpcx.OptionalTime(key.LastUsedAt),
		ExpiresAt:     grpcx.OptionalTime(key.ExpiresAt),
		CreatedAt:     grpcx.Time(key.CreatedAt),
		UpdatedAt:     grpcx.OptionalTime(key.UpdatedAt),
		DeletedAt:     grpcx.OptionalTime(key.DeletedAt),
	}
}
