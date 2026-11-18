package grpc_controllers

import (
	"context"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/restful/runtime/useCases"
	"github.com/murkio-dev/vault/pkg/grpcx"
	"github.com/murkio-dev/vault/pkg/httpx"
	vaultv1 "github.com/murkio-dev/vault/pkg/rpc/vaultv1"
	"google.golang.org/grpc"
)

type RegisterInput struct {
	Registrar  grpc.ServiceRegistrar
	EnvUseCase contracts.IUseCase[useCases.ReadEnvInput, useCases.ReadEnvOutput]
}

func RegisterControllers(input RegisterInput) error {
	err := httpx.AssertNotNilParams(map[string]any{
		"input.Registrar":  input.Registrar,
		"input.EnvUseCase": input.EnvUseCase,
	})
	if err != nil {
		return err
	}

	vaultv1.RegisterVaultServiceServer(input.Registrar, controller{envUseCase: input.EnvUseCase})

	return nil
}

type controller struct {
	vaultv1.UnimplementedVaultServiceServer

	envUseCase contracts.IUseCase[useCases.ReadEnvInput, useCases.ReadEnvOutput]
}

func (c controller) ReadEnv(ctx context.Context, req *vaultv1.ReadEnvRequest) (*vaultv1.ReadEnvResponse, error) {

	output := c.envUseCase.Perform(ctx, useCases.ReadEnvInput{
		Requester: grpcx.Requester(ctx),
		UriID:     req.GetUriId(),
		Names:     req.GetNames(),
	})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
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
