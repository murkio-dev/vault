package useCases

import (
	"context"
	"database/sql"
	"errors"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/sqlite"
)

// Builder

type UpdateApplicationUseCase struct {
	Updater contracts.IApplicationUpdater
	Guard   contracts.IAccessGuard
	Tx      contracts.IAuditedTx
}

func NewUpdateApplicationUseCase(updater contracts.IApplicationUpdater, guard contracts.IAccessGuard, tx contracts.IAuditedTx) UpdateApplicationUseCase {
	return UpdateApplicationUseCase{
		Updater: updater,
		Guard:   guard,
		Tx:      tx,
	}
}

// DTOS

type UpdateInput struct {
	Requester     entities.ApiHeader
	Applications  []entities.Application
	DuplicatedIDs map[uuid.UUID]bool
}

type UpdateOutput struct {
	Data   []entities.Application `json:"data,omitempty"`
	Errors []entities.Error       `json:"errors,omitempty"`
}

// UseCase

func (u UpdateApplicationUseCase) Perform(ctx context.Context, input UpdateInput) UpdateOutput {
	var errs []entities.Error

	data := make([]entities.Application, 0, len(input.Applications))

	duplicated := input.DuplicatedIDs
	reported := make(map[uuid.UUID]bool)

	for _, application := range input.Applications {
		if application.ID == (uuid.UUID{}) {
			errs = append(errs, entities.InvalidApplicationIDError(application.ID))
			continue
		}

		if duplicated[application.ID] {
			if !reported[application.ID] {
				reported[application.ID] = true
				errs = append(errs, entities.DuplicateApplicationIDError(application.ID))
			}
			continue
		}

		if err := application.Validate(); err != nil {
			errs = append(errs, entities.InvalidApplicationNameError(err, &application.Name))
			continue
		}

		if denied := u.Guard.Perform(ctx, entities.AuthorizeInput{Requester: input.Requester, Application: application.ID, Action: entities.ActionApplicationUpdate}).Error; denied != nil {
			errs = append(errs, *denied)
			continue
		}

		var value entities.Application
		entry := entities.NewAuditLog(input.Requester, application.ID, entities.AuditApplicationUpdated, entities.AuditAllowed,
			entities.AuditTarget{Type: "application", ID: application.ID.String()}, map[string]any{"name": application.Name})

		err := u.Tx.Run(ctx, entry, func(ctx context.Context) (err error) {
			value, err = u.Updater.Update(ctx, application)
			return err
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				errs = append(errs, entities.ApplicationNotFoundError(application.ID))
				continue
			}
			if sqlite.IsUniqueViolation(err) {
				errs = append(errs, entities.ApplicationAlreadyExistsError(application.Name))
				continue
			}

			errs = append(errs, entities.InternalServerError(err, &application.Name))
			continue
		}

		data = append(data, value)
	}

	if len(data) == 0 {
		data = nil
	}

	return UpdateOutput{
		Data:   data,
		Errors: errs,
	}
}
