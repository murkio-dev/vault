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

type AuthorizeUseCase struct {
	Finder     contracts.IMemberFinder
	UserFinder contracts.IMemberUserFinder
	Audit      contracts.IAuditLogCreator
}

func NewAuthorizeUseCase(finder contracts.IMemberFinder, userFinder contracts.IMemberUserFinder, audit contracts.IAuditLogCreator) AuthorizeUseCase {
	return AuthorizeUseCase{Finder: finder, UserFinder: userFinder, Audit: audit}
}

var _ contracts.IAccessGuard = AuthorizeUseCase{}

// UseCase

// Perform é o porteiro de toda ação autenticada, compartilhado com os outros
// services pelo initializr raiz. Carrega o vínculo do ator com a aplicação a
// cada chamada, no banco, porque remover ou suspender um membro precisa valer
// na hora, mesmo com o JWT ainda dentro do prazo, e aplica entities.Decide.
//
// Se negar, registra "access.denied" na auditoria e devolve 401 (sem
// credencial), 404 (quem não é membro não pode saber o que existe) ou 403 (é
// membro, mas não pode). Falhar ao gravar a negação não muda a resposta: a
// ação continua negada. Se permitir e OwnerMustExist, confere que o dono
// existe na aplicação (senão 404, igual ao que não se pode ver) e devolve o
// dono resolvido, com SelfOwner trocado pelo membro do ator.
//
// Exemplo: um developer pedindo a tabela de outro membro recebe 403 e uma
// linha access.denied; alguém de fora da aplicação recebe 404 e a mesma linha.
func (u AuthorizeUseCase) Perform(ctx context.Context, input entities.AuthorizeInput) entities.AuthorizeOutput {

	actor := input.Requester.Actor

	var member *entities.Member
	if actor.Authenticated() && input.Application != (uuid.UUID{}) {
		found, err := u.find(ctx, actor, input.Application)
		switch {
		case err == nil:
			member = &found
		case errors.Is(err, sql.ErrNoRows):
		default:
			internal := entities.InternalServerError(err, nil)
			return entities.AuthorizeOutput{Error: &internal}
		}
	}

	owner := input.Owner
	if owner == entities.SelfOwner && member != nil {
		owner = member.ID
	}

	var failure entities.Error
	switch entities.Decide(actor, member, input.Action, owner) {
	case entities.Allow:
		return u.allowed(ctx, input, member, owner)
	case entities.Hidden:
		failure = entities.NotFoundError("Resource")
	case entities.Forbidden:
		failure = entities.ForbiddenError()
	default:
		failure = entities.UnauthorizedError()
	}

	if actor.Authenticated() {
		denied := entities.NewAuditLog(input.Requester, input.Application, entities.AuditAccessDenied, entities.AuditDenied,
			entities.AuditTarget{}, map[string]any{"action": string(input.Action), "status": failure.Status})
		_, _ = u.Audit.Create(ctx, denied)
	}

	return entities.AuthorizeOutput{Error: &failure}
}

func (u AuthorizeUseCase) allowed(ctx context.Context, input entities.AuthorizeInput, member *entities.Member, owner uuid.UUID) entities.AuthorizeOutput {

	output := entities.AuthorizeOutput{Owner: owner}
	if member != nil {
		output.Member = *member
	}

	if !input.OwnerMustExist {
		return output
	}

	notFound := entities.NotFoundError("Resource")
	if owner == (uuid.UUID{}) || owner == entities.SelfOwner {
		output.Error = &notFound
		return output
	}

	if _, err := u.Finder.FindById(ctx, input.Application, owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			output.Error = &notFound
			return output
		}
		internal := entities.InternalServerError(err, nil)
		output.Error = &internal
	}

	return output
}

// find carrega o vínculo do ator: o token do SDK é de um membro conhecido; a
// pessoa é achada pela conta, entre os membros da aplicação.
func (u AuthorizeUseCase) find(ctx context.Context, actor entities.Actor, application uuid.UUID) (entities.Member, error) {
	if actor.Kind == entities.ActorToken {
		return u.Finder.FindById(ctx, application, actor.MemberID)
	}

	return u.UserFinder.FindByUser(ctx, application, actor.UserID)
}
