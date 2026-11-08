package useCases

import (
	"context"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/sqlite"
)

// Builder

type CreateApplicationUseCase struct {
	Checker       contracts.IApplicationChecker
	Creator       contracts.IApplicationCreator
	MemberCreator contracts.IMemberCreator
	Guard         contracts.IAccessGuard
	Tx            contracts.IAuditedTx
}

func NewCreateApplicationUseCase(checker contracts.IApplicationChecker, creator contracts.IApplicationCreator, memberCreator contracts.IMemberCreator, guard contracts.IAccessGuard, tx contracts.IAuditedTx) CreateApplicationUseCase {
	return CreateApplicationUseCase{
		Checker:       checker,
		Creator:       creator,
		MemberCreator: memberCreator,
		Guard:         guard,
		Tx:            tx,
	}
}

// DTOS

type CreateInput struct {
	Requester   entities.ApiHeader
	Application []entities.Application
}

type CreateOutput struct {
	Data   []entities.Application `json:"data,omitempty"`
	Errors []entities.Error       `json:"errors,omitempty"`
}

// UseCase

func (u CreateApplicationUseCase) Perform(ctx context.Context, input CreateInput) CreateOutput {

	if denied := u.Guard.Perform(ctx, entities.AuthorizeInput{Requester: input.Requester, Application: entities.Application{}.ID, Action: entities.ActionApplicationCreate, Owner: entities.Application{}.ID}).Error; denied != nil {
		return CreateOutput{Errors: []entities.Error{*denied}}
	}

	var errs []entities.Error

	data := make([]entities.Application, 0, len(input.Application))

	for _, application := range input.Application {
		entity, err := entities.NewApplication(application.Name, input.Requester.Actor.UserID)
		if err != nil {
			errs = append(errs, entities.InvalidApplicationNameError(err, &application.Name))
			continue
		}

		exists, err := u.Checker.Exist(ctx, entity)
		if err != nil {
			errs = append(errs, entities.InternalServerError(err, &application.Name))
			continue
		}
		if exists {
			errs = append(errs, entities.ApplicationAlreadyExistsError(application.Name))
			continue
		}

		admin := entities.NewMember(entity.ID, entity.CreatedBy, entities.RoleAdmin, nil)
		entry := entities.NewAuditLog(input.Requester, entity.ID, entities.AuditApplicationCreated, entities.AuditAllowed,
			entities.AuditTarget{Type: "application", ID: entity.ID.String()},
			map[string]any{"name": entity.Name, "admin_member_id": admin.ID.String()})

		err = u.Tx.Run(ctx, entry, func(ctx context.Context) error {
			if _, err := u.Creator.Create(ctx, entity); err != nil {
				return err
			}
			_, err := u.MemberCreator.Create(ctx, admin)
			return err
		})
		if err != nil {
			if sqlite.IsUniqueViolation(err) {
				errs = append(errs, entities.ApplicationAlreadyExistsError(application.Name))
				continue
			}
			errs = append(errs, entities.InternalServerError(err, &application.Name))
			continue
		}

		data = append(data, entity)
	}

	if len(data) == 0 {
		data = nil
	}

	return CreateOutput{
		Data:   data,
		Errors: errs,
	}
}
