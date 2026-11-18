package grpc_controllers

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/token/useCases"
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
}

func RegisterControllers(input RegisterInput) error {
	err := httpx.AssertNotNilParams(map[string]any{
		"input.Registrar":     input.Registrar,
		"input.CreateUseCase": input.CreateUseCase,
		"input.ListUseCase":   input.ListUseCase,
		"input.RevokeUseCase": input.RevokeUseCase,
	})
	if err != nil {
		return err
	}

	vaultv1.RegisterTokenServiceServer(input.Registrar, controller{
		createUseCase: input.CreateUseCase,
		listUseCase:   input.ListUseCase,
		revokeUseCase: input.RevokeUseCase,
	})

	return nil
}

type controller struct {
	vaultv1.UnimplementedTokenServiceServer

	createUseCase contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]
	listUseCase   contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
	revokeUseCase contracts.IUseCase[useCases.RevokeInput, useCases.RevokeOutput]
}

func (c controller) CreateToken(ctx context.Context, req *vaultv1.CreateTokenRequest) (*vaultv1.CreateTokenResponse, error) {

	application, member, e := parseOwner(req.GetApplicationId(), req.GetMemberId())
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	input := useCases.CreateInput{Requester: grpcx.Requester(ctx), Application: application, Member: member, Name: req.GetName()}
	if req.GetExpiresAt() != nil {
		expiresAt := req.GetExpiresAt().AsTime()
		input.ExpiresAt = &expiresAt
	}

	output := c.createUseCase.Perform(ctx, input)
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return &vaultv1.CreateTokenResponse{AccessToken: toMessage(output.Data.AccessToken), Token: output.Data.Token, Uri: output.Data.URI}, nil
}

func (c controller) ListTokens(ctx context.Context, req *vaultv1.ListTokensRequest) (*vaultv1.ListTokensResponse, error) {

	application, member, e := parseOwner(req.GetApplicationId(), req.GetMemberId())
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.listUseCase.Perform(ctx, useCases.ListInput{Requester: grpcx.Requester(ctx), Application: application, Member: member})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	tokens := make([]*vaultv1.AccessToken, len(output.Data))
	for i, token := range output.Data {
		tokens[i] = toMessage(token)
	}

	return &vaultv1.ListTokensResponse{AccessTokens: tokens}, nil
}

func (c controller) RevokeToken(ctx context.Context, req *vaultv1.RevokeTokenRequest) (*vaultv1.RevokeTokenResponse, error) {

	application, member, e := parseOwner(req.GetApplicationId(), req.GetMemberId())
	if e != nil {
		return nil, grpcx.Error(*e)
	}
	token, e := grpcx.UUID(req.GetTokenId(), "token_id")
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.revokeUseCase.Perform(ctx, useCases.RevokeInput{Requester: grpcx.Requester(ctx), Application: application, Member: member, Token: token})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return &vaultv1.RevokeTokenResponse{}, nil
}

func parseOwner(rawApplication, rawMember string) (application, member uuid.UUID, e *entities.Error) {
	if application, e = grpcx.UUID(rawApplication, "application_id"); e != nil {
		return
	}
	member, e = grpcx.MemberID(rawMember)
	return
}

func toMessage(token entities.AccessToken) *vaultv1.AccessToken {
	return &vaultv1.AccessToken{
		Id:            token.ID.String(),
		ApplicationId: token.ApplicationID.String(),
		MemberId:      token.MemberID.String(),
		UriId:         token.UriID,
		Name:          token.Name,
		Prefix:        token.Prefix,
		ExpiresAt:     grpcx.OptionalTime(token.ExpiresAt),
		LastUsedAt:    grpcx.OptionalTime(token.LastUsedAt),
		RevokedAt:     grpcx.OptionalTime(token.RevokedAt),
		CreatedAt:     grpcx.Time(token.CreatedAt),
	}
}
