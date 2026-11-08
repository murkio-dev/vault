package useCases

import (
	"context"
	"database/sql"
	"errors"

	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
)

// Builder

type DeleteApplicationUseCase struct {
	Deleter contracts.IApplicationDeleter
	Guard   contracts.IAccessGuard
	Tx      contracts.IAuditedTx
}

func NewDeleteApplicationUseCase(deleter contracts.IApplicationDeleter, guard contracts.IAccessGuard, tx contracts.IAuditedTx) DeleteApplicationUseCase {
	return DeleteApplicationUseCase{
		Deleter: deleter,
		Guard:   guard,
		Tx:      tx,
	}
}

// DTOS

type DeleteInput struct {
	Requester     entities.ApiHeader
	IDs           []uuid.UUID
	DuplicatedIDs map[uuid.UUID]bool
}

type DeleteOutput struct {
	Data   []uuid.UUID      `json:"data,omitempty"`
	Errors []entities.Error `json:"errors,omitempty"`
}

// UseCase

func (u DeleteApplicationUseCase) Perform(ctx context.Context, input DeleteInput) DeleteOutput {
	var errs []entities.Error

	data := make([]uuid.UUID, 0, len(input.IDs))

	duplicated := input.DuplicatedIDs
	reported := make(map[uuid.UUID]bool)

	for _, id := range input.IDs {
		if id == (uuid.UUID{}) {
			errs = append(errs, entities.InvalidApplicationIDError(id))
			continue
		}

		if duplicated[id] {
			if !reported[id] {
				reported[id] = true
				errs = append(errs, entities.DuplicateApplicationIDError(id))
			}
			continue
		}

		if denied := u.Guard.Perform(ctx, entities.AuthorizeInput{Requester: input.Requester, Application: id, Action: entities.ActionApplicationDelete}).Error; denied != nil {
			errs = append(errs, *denied)
			continue
		}

		entry := entities.NewAuditLog(input.Requester, id, entities.AuditApplicationDeleted, entities.AuditAllowed,
			entities.AuditTarget{Type: "application", ID: id.String()}, nil)

		err := u.Tx.Run(ctx, entry, func(ctx context.Context) error {
			return u.Deleter.Delete(ctx, id)
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				errs = append(errs, entities.ApplicationNotFoundError(id))
				continue
			}

			errs = append(errs, entities.InternalServerError(err, nil))
			continue
		}

		data = append(data, id)
	}

	if len(data) == 0 {
		data = nil
	}

	return DeleteOutput{
		Data:   data,
		Errors: errs,
	}
}
