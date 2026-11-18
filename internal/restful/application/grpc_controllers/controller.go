package grpc_controllers

import (
	"context"
	"errors"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/application/services"
	"github.com/murkio-dev/vault/internal/restful/application/useCases"
	"github.com/murkio-dev/vault/pkg/grpcx"
	"github.com/murkio-dev/vault/pkg/httpx"
	vaultv1 "github.com/murkio-dev/vault/pkg/rpc/vaultv1"
	"google.golang.org/grpc"
)

type RegisterInput struct {
	Registrar      grpc.ServiceRegistrar
	CreateUseCase  contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]
	GetByIDUseCase contracts.IUseCase[useCases.GetByIDInput, useCases.GetByIDOutput]
	ListUseCase    contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
	UpdateUseCase  contracts.IUseCase[useCases.UpdateInput, useCases.UpdateOutput]
	DeleteUseCase  contracts.IUseCase[useCases.DeleteInput, useCases.DeleteOutput]
}

func RegisterControllers(input RegisterInput) error {
	err := httpx.AssertNotNilParams(map[string]any{
		"input.Registrar":     input.Registrar,
		"input.CreateUseCase": input.CreateUseCase,
		"input.GetByID":       input.GetByIDUseCase,
		"input.List":          input.ListUseCase,
		"input.UpdateUseCase": input.UpdateUseCase,
		"input.DeleteUseCase": input.DeleteUseCase,
	})
	if err != nil {
		return err
	}

	vaultv1.RegisterApplicationServiceServer(input.Registrar, controller{
		createUseCase:  input.CreateUseCase,
		getByIDUseCase: input.GetByIDUseCase,
		listUseCase:    input.ListUseCase,
		updateUseCase:  input.UpdateUseCase,
		deleteUseCase:  input.DeleteUseCase,
	})

	return nil
}

type controller struct {
	vaultv1.UnimplementedApplicationServiceServer

	createUseCase  contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]
	getByIDUseCase contracts.IUseCase[useCases.GetByIDInput, useCases.GetByIDOutput]
	listUseCase    contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
	updateUseCase  contracts.IUseCase[useCases.UpdateInput, useCases.UpdateOutput]
	deleteUseCase  contracts.IUseCase[useCases.DeleteInput, useCases.DeleteOutput]
}

func (c controller) CreateApplications(ctx context.Context, req *vaultv1.CreateApplicationsRequest) (*vaultv1.ApplicationsResponse, error) {

	drafts := req.GetApplications()
	if len(drafts) == 0 {
		return nil, grpcx.Error(entities.InvalidRequestBodyError(errors.New("empty payload")))
	}

	applications := make([]entities.Application, len(drafts))
	for i, draft := range drafts {
		applications[i] = entities.Application{Name: draft.GetName()}
	}

	output := c.createUseCase.Perform(ctx, useCases.CreateInput{
		Requester:   grpcx.Requester(ctx),
		Application: applications,
	})

	return applicationsResponse(output.Data, output.Errors)
}

func (c controller) GetApplication(ctx context.Context, req *vaultv1.GetApplicationRequest) (*vaultv1.Application, error) {

	id, e := services.ParseID(req.GetId())
	if e != nil {
		return nil, grpcx.Error(*e)
	}

	output := c.getByIDUseCase.Perform(ctx, useCases.GetByIDInput{
		Requester:   grpcx.Requester(ctx),
		Application: id,
	})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return toMessage(output.Data), nil
}

func (c controller) ListApplications(ctx context.Context, req *vaultv1.ListApplicationsRequest) (*vaultv1.ListApplicationsResponse, error) {

	output := c.listUseCase.Perform(ctx, useCases.ListInput{
		Requester: grpcx.Requester(ctx),
		Page:      int(req.GetPage()),
		Limit:     int(req.GetLimit()),
	})
	if output.Error != nil {
		return nil, grpcx.Error(*output.Error)
	}

	return &vaultv1.ListApplicationsResponse{Applications: toMessages(output.Data), Page: grpcx.Page(output.Page)}, nil
}

func (c controller) UpdateApplications(ctx context.Context, req *vaultv1.UpdateApplicationsRequest) (*vaultv1.ApplicationsResponse, error) {

	changes := req.GetApplications()
	if len(changes) == 0 {
		return nil, grpcx.Error(entities.InvalidRequestBodyError(errors.New("empty payload")))
	}

	applications := make([]entities.Application, len(changes))
	for i, change := range changes {
		id, e := services.ParseID(change.GetId())
		if e != nil {
			return nil, grpcx.Error(*e)
		}
		applications[i] = entities.Application{ID: id, Name: change.GetName()}
	}

	output := c.updateUseCase.Perform(ctx, useCases.UpdateInput{
		Requester:     grpcx.Requester(ctx),
		Applications:  applications,
		DuplicatedIDs: services.DuplicatedIDs(services.ApplicationIDs(applications)),
	})

	return applicationsResponse(output.Data, output.Errors)
}

func (c controller) DeleteApplications(ctx context.Context, req *vaultv1.DeleteApplicationsRequest) (*vaultv1.DeleteApplicationsResponse, error) {

	raws := req.GetIds()
	if len(raws) == 0 {
		return nil, grpcx.Error(entities.InvalidRequestBodyError(errors.New("empty payload")))
	}

	ids := make([]uuid.UUID, len(raws))
	for i, raw := range raws {
		id, e := services.ParseID(raw)
		if e != nil {
			return nil, grpcx.Error(*e)
		}
		ids[i] = id
	}

	output := c.deleteUseCase.Perform(ctx, useCases.DeleteInput{
		Requester:     grpcx.Requester(ctx),
		IDs:           ids,
		DuplicatedIDs: services.DuplicatedIDs(ids),
	})
	if len(output.Data) == 0 && len(output.Errors) > 0 {
		return nil, grpcx.Error(output.Errors[0])
	}

	deleted := make([]string, len(output.Data))
	for i, id := range output.Data {
		deleted[i] = id.String()
	}

	return &vaultv1.DeleteApplicationsResponse{Ids: deleted, Errors: grpcx.ItemErrors(output.Errors)}, nil
}

// applicationsResponse monta a resposta de uma escrita em lote, com a mesma
// regra do httpx.Respond da entrega HTTP, para as duas entregas se comportarem
// igual. Recebe os itens que deram certo e os erros dos que falharam.
// Sem nenhum sucesso e com erro, devolve o status gRPC do primeiro erro e
// nenhuma resposta. Com ao menos um sucesso, devolve a resposta com as
// aplicações e, em errors, um erro por item que falhou (o 207 do HTTP).
// Exemplo: 2 sucessos e 1 conflito devolvem OK com 2 aplicações e 1 erro; 0
// sucessos e 1 conflito devolvem AlreadyExists.
// Lote vazio nunca chega aqui: os handlers recusam antes de chamar o use case.
func applicationsResponse(data []entities.Application, errs []entities.Error) (*vaultv1.ApplicationsResponse, error) {
	if len(data) == 0 && len(errs) > 0 {
		return nil, grpcx.Error(errs[0])
	}

	return &vaultv1.ApplicationsResponse{Applications: toMessages(data), Errors: grpcx.ItemErrors(errs)}, nil
}

func toMessages(applications []entities.Application) []*vaultv1.Application {
	messages := make([]*vaultv1.Application, len(applications))
	for i, application := range applications {
		messages[i] = toMessage(application)
	}
	return messages
}

func toMessage(application entities.Application) *vaultv1.Application {
	return &vaultv1.Application{
		Id:        application.ID.String(),
		Name:      application.Name,
		CreatedBy: application.CreatedBy.String(),
		CreatedAt: grpcx.Time(application.CreatedAt),
		UpdatedAt: grpcx.OptionalTime(application.UpdatedAt),
	}
}
