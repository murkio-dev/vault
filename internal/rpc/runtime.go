package rpc

import (
	"context"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/restful/runtime/useCases"
	vaultv1 "github.com/murkio-dev/vault/pkg/rpc/vaultv1"
)

type ReadEnvUseCase = contracts.IUseCase[useCases.ReadEnvInput, useCases.ReadEnvOutput]

type VaultController struct {
	vaultv1.UnimplementedVaultServiceServer

	EnvUseCase ReadEnvUseCase
}

func NewVaultController(env ReadEnvUseCase) *VaultController {
	return &VaultController{EnvUseCase: env}
}

func (c *VaultController) ReadEnv(ctx context.Context, req *vaultv1.ReadEnvRequest) (*vaultv1.ReadEnvResponse, error) {

	output := c.EnvUseCase.Perform(ctx, useCases.ReadEnvInput{
		Requester: requester(ctx),
		UriID:     req.GetUriId(),
		Names:     req.GetNames(),
	})
	if output.Error != nil {
		return nil, toStatus(*output.Error)
	}

	response := &vaultv1.ReadEnvResponse{
		UriId:     output.Data.UriID,
		Variables: make([]*vaultv1.Variable, 0, len(output.Data.Variables)),
	}
	for _, variable := range output.Data.Variables {
		response.Variables = append(response.Variables, &vaultv1.Variable{
			Name:    variable.Name,
			Type:    string(variable.Type),
			Version: int32(variable.Version),
			Value:   variable.Value,
		})
	}

	return response, nil
}
