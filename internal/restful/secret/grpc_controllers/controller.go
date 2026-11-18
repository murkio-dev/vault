package grpc_controllers

import (
	"context"
	"errors"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/secret/useCases"
	"github.com/murkio-dev/vault/pkg/grpcx"
	"github.com/murkio-dev/vault/pkg/httpx"
	vaultv1 "github.com/murkio-dev/vault/pkg/rpc/vaultv1"
	"google.golang.org/grpc"
)

type RegisterInput struct {
	Registrar       grpc.ServiceRegistrar
	CreateUseCase   contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]
	ListUseCase     contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
	GetUseCase      contracts.IUseCase[useCases.GetInput, useCases.GetOutput]
	UpdateUseCase   contracts.IUseCase[useCases.UpdateInput, useCases.UpdateOutput]
	RenameUseCase   contracts.IUseCase[useCases.RenameInput, useCases.RenameOutput]
	DeleteUseCase   contracts.IUseCase[useCases.DeleteInput, useCases.DeleteOutput]
	VersionsUseCase contracts.IUseCase[useCases.VersionsInput, useCases.VersionsOutput]
	RevealUseCase   contracts.IUseCase[useCases.RevealInput, useCases.RevealOutput]
}

func RegisterControllers(input RegisterInput) error {
	err := httpx.AssertNotNilParams(map[string]any{
		"input.Registrar":       input.Registrar,
		"input.CreateUseCase":   input.CreateUseCase,
		"input.ListUseCase":     input.ListUseCase,
		"input.GetUseCase":      input.GetUseCase,
		"input.UpdateUseCase":   input.UpdateUseCase,
		"input.RenameUseCase":   input.RenameUseCase,
		"input.DeleteUseCase":   input.DeleteUseCase,
		"input.VersionsUseCase": input.VersionsUseCase,
		"input.RevealUseCase":   input.RevealUseCase,
	})
	if err != nil {
		return err
	}

	vaultv1.RegisterSecretServiceServer(input.Registrar, controller{
		createUseCase:   input.CreateUseCase,
		listUseCase:     input.ListUseCase,
		getUseCase:      input.GetUseCase,
		updateUseCase:   input.UpdateUseCase,
		renameUseCase:   input.RenameUseCase,
		deleteUseCase:   input.DeleteUseCase,
		versionsUseCase: input.VersionsUseCase,
		revealUseCase:   input.RevealUseCase,
	})

	return nil
}

type controller struct {
	vaultv1.UnimplementedSecretServiceServer

	createUseCase   contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]
	listUseCase     contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
	getUseCase      contracts.IUseCase[useCases.GetInput, useCases.GetOutput]
	updateUseCase   contracts.IUseCase[useCases.UpdateInput, useCases.UpdateOutput]
	renameUseCase   contracts.IUseCase[useCases.RenameInput, useCases.RenameOutput]
	deleteUseCase   contracts.IUseCase[useCases.DeleteInput, useCases.DeleteOutput]
	versionsUseCase contracts.IUseCase[useCases.VersionsInput, useCases.VersionsOutput]
	revealUseCase   contracts.IUseCase[useCases.RevealInput, useCases.RevealOutput]
}

func (c controller) CreateSecrets(ctx context.Context, req *vaultv1.CreateSecretsRequest) (*vaultv1.CreateSecretsResponse, error) {

	application, member, e := parseOwner(req.GetApplicationId(), req.GetMemberId())
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	drafts := req.GetItems()
	if len(drafts) == 0 {
		return nil, grpcx.Error(entities.InvalidRequestBodyError(errors.New("empty payload")))
	}

	items := make([]entities.SecretDraft, len(drafts))
	for i, draft := range drafts {
		item := entities.SecretDraft{
			Name:   draft.GetName(),
			Type:   entities.SecretType(draft.GetType()),
			Source: entities.SecretSource(draft.GetSource()),
			Value:  draft.GetValue(),
		}
		if draft.ReferenceSecretId != nil {
			reference, e := grpcx.UUID(draft.GetReferenceSecretId(), "reference_secret_id")
			if e != nil {
				return nil, grpcx.Error(*e)
			}
			item.ReferenceSecretID = &reference
		}
		items[i] = item
	}

	output := c.createUseCase.Perform(ctx, useCases.CreateInput{Requester: grpcx.Requester(ctx), Application: application, Member: member, Items: items})
	if len(output.Data) == 0 && len(output.Errors) > 0 {
		return nil, grpcx.Error(output.Errors[0])
	}

	return &vaultv1.CreateSecretsResponse{Secrets: toMessages(output.Data), Errors: grpcx.ItemErrors(output.Errors)}, nil
}

func (c controller) ListSecrets(ctx context.Context, req *vaultv1.ListSecretsRequest) (*vaultv1.ListSecretsResponse, error) {

	application, member, e := parseOwner(req.GetApplicationId(), req.GetMemberId())
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.listUseCase.Perform(ctx, useCases.ListInput{Requester: grpcx.Requester(ctx), Application: application, Member: member})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return &vaultv1.ListSecretsResponse{Secrets: toMessages(output.Data)}, nil
}

func (c controller) GetSecret(ctx context.Context, req *vaultv1.SecretRequest) (*vaultv1.Secret, error) {

	application, member, secret, e := parseSecret(req.GetApplicationId(), req.GetMemberId(), req.GetSecretId())
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.getUseCase.Perform(ctx, useCases.GetInput{Requester: grpcx.Requester(ctx), Application: application, Member: member, Secret: secret})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return toMessage(output.Data), nil
}

func (c controller) UpdateSecret(ctx context.Context, req *vaultv1.UpdateSecretRequest) (*vaultv1.Secret, error) {

	application, member, secret, e := parseSecret(req.GetApplicationId(), req.GetMemberId(), req.GetSecretId())
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.updateUseCase.Perform(ctx, useCases.UpdateInput{
		Requester: grpcx.Requester(ctx), Application: application, Member: member, Secret: secret, Value: req.GetValue(),
	})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return toMessage(output.Data), nil
}

func (c controller) RenameSecret(ctx context.Context, req *vaultv1.RenameSecretRequest) (*vaultv1.Secret, error) {

	application, member, secret, e := parseSecret(req.GetApplicationId(), req.GetMemberId(), req.GetSecretId())
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.renameUseCase.Perform(ctx, useCases.RenameInput{
		Requester: grpcx.Requester(ctx), Application: application, Member: member, Secret: secret, Name: req.GetName(),
	})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return toMessage(output.Data), nil
}

func (c controller) DeleteSecret(ctx context.Context, req *vaultv1.SecretRequest) (*vaultv1.DeleteSecretResponse, error) {

	application, member, secret, e := parseSecret(req.GetApplicationId(), req.GetMemberId(), req.GetSecretId())
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.deleteUseCase.Perform(ctx, useCases.DeleteInput{Requester: grpcx.Requester(ctx), Application: application, Member: member, Secret: secret})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return &vaultv1.DeleteSecretResponse{}, nil
}

func (c controller) ListSecretVersions(ctx context.Context, req *vaultv1.SecretRequest) (*vaultv1.ListSecretVersionsResponse, error) {

	application, member, secret, e := parseSecret(req.GetApplicationId(), req.GetMemberId(), req.GetSecretId())
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.versionsUseCase.Perform(ctx, useCases.VersionsInput{Requester: grpcx.Requester(ctx), Application: application, Member: member, Secret: secret})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	versions := make([]*vaultv1.SecretVersion, len(output.Data))
	for i, version := range output.Data {
		versions[i] = &vaultv1.SecretVersion{
			Version:   int32(version.Version),
			CreatedBy: version.CreatedBy.String(),
			CreatedAt: grpcx.Time(version.CreatedAt),
		}
	}

	return &vaultv1.ListSecretVersionsResponse{Versions: versions}, nil
}

func (c controller) RevealSecret(ctx context.Context, req *vaultv1.RevealSecretRequest) (*vaultv1.RevealedSecret, error) {

	application, member, secret, e := parseSecret(req.GetApplicationId(), req.GetMemberId(), req.GetSecretId())
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.revealUseCase.Perform(ctx, useCases.RevealInput{
		Requester: grpcx.Requester(ctx), Application: application, Member: member, Secret: secret, Version: int(req.GetVersion()),
	})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return &vaultv1.RevealedSecret{
		Id:      output.Data.ID.String(),
		Name:    output.Data.Name,
		Version: int32(output.Data.Version),
		Value:   output.Data.Value,
	}, nil
}

func parseOwner(rawApplication, rawMember string) (application, member uuid.UUID, e *entities.Error) {
	if application, e = grpcx.UUID(rawApplication, "application_id"); e != nil {
		return
	}
	member, e = grpcx.MemberID(rawMember)
	return
}

func parseSecret(rawApplication, rawMember, rawSecret string) (application, member, secret uuid.UUID, e *entities.Error) {
	if application, member, e = parseOwner(rawApplication, rawMember); e != nil {
		return
	}
	secret, e = grpcx.UUID(rawSecret, "secret_id")
	return
}

func toMessages(secrets []entities.Secret) []*vaultv1.Secret {
	messages := make([]*vaultv1.Secret, len(secrets))
	for i, secret := range secrets {
		messages[i] = toMessage(secret)
	}
	return messages
}

func toMessage(secret entities.Secret) *vaultv1.Secret {
	return &vaultv1.Secret{
		Id:                secret.ID.String(),
		ApplicationId:     secret.ApplicationID.String(),
		MemberId:          secret.MemberID.String(),
		Name:              secret.Name,
		Type:              string(secret.Type),
		Source:            string(secret.Source),
		ReferenceSecretId: grpcx.OptionalUUID(secret.ReferenceSecretID),
		Version:           int32(secret.Version),
		CreatedBy:         secret.CreatedBy.String(),
		CreatedAt:         grpcx.Time(secret.CreatedAt),
		UpdatedAt:         grpcx.OptionalTime(secret.UpdatedAt),
	}
}
