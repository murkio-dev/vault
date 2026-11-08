package entities

import "uuid"

type Action string

const (
	ActionApplicationCreate Action = "application.create"
	ActionApplicationRead   Action = "application.read"
	ActionApplicationUpdate Action = "application.update"
	ActionApplicationDelete Action = "application.delete"

	ActionMemberList   Action = "member.list"
	ActionMemberUpdate Action = "member.update"
	ActionMemberRemove Action = "member.remove"

	ActionInvitationManage Action = "invitation.manage"
	ActionInvitationAccept Action = "invitation.accept"

	ActionSecretRead   Action = "secret.read"
	ActionSecretWrite  Action = "secret.write"
	ActionSecretReveal Action = "secret.reveal"

	ActionTokenCreate Action = "token.create"
	ActionTokenList   Action = "token.list"
	ActionTokenRevoke Action = "token.revoke"

	ActionAuditRead Action = "audit.read"
	ActionEnvRead   Action = "env.read"
)

var knownActions = map[Action]bool{
	ActionApplicationCreate: true, ActionApplicationRead: true, ActionApplicationUpdate: true, ActionApplicationDelete: true,
	ActionMemberList: true, ActionMemberUpdate: true, ActionMemberRemove: true,
	ActionInvitationManage: true, ActionInvitationAccept: true,
	ActionSecretRead: true, ActionSecretWrite: true, ActionSecretReveal: true,
	ActionTokenCreate: true, ActionTokenList: true, ActionTokenRevoke: true,
	ActionAuditRead: true, ActionEnvRead: true,
}

// known protege contra o typo e contra a ação nova que esqueceram de decidir:
// o que não está na lista nega, até para o admin e o platform admin. Sem isso,
// uma Action("secret.revela") digitada errada passaria pelo "admin pode tudo".
// Cada ação nova precisa entrar em knownActions e na matriz de Decide, e o
// teste da matriz falha se faltar. Exemplo: Action("") e Action("x") negam.
func (a Action) known() bool {
	return knownActions[a]
}

var SelfOwner = uuid.UUID{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f, 0xff, 0xbf, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

type Decision int

const (
	Deny Decision = iota
	Allow
	// Hidden: quem pediu não pode nem saber que o recurso existe (não é membro).
	// O use case responde 404.
	Hidden
	// Forbidden: é membro, mas não pode a ação. O use case responde 403.
	Forbidden
)

// Decide diz se actor pode fazer action. member é o vínculo do ator com a
// aplicação (nil se não for membro) e owner é o dono do recurso (o membro dono
// da tabela de segredos, do token ou da trilha); zerado quando o recurso é da
// aplicação inteira.
//
// Regras: o platform admin pode tudo; o admin da aplicação pode tudo nela; o
// developer só mexe no que é dele (owner == o próprio membro) e lê a
// aplicação; o token do SDK só lê o env. Membro suspenso ou ausente não vê
// nada. Qualquer ação fora da lista nega, para qualquer ator.
func Decide(actor Actor, member *Member, action Action, owner uuid.UUID) Decision {

	if !actor.Authenticated() || !action.known() {
		return Deny
	}

	if actor.Kind == ActorToken {
		if action == ActionEnvRead && member != nil && member.Active() && member.ID == actor.MemberID {
			return Allow
		}
		return Forbidden
	}

	if action == ActionApplicationCreate || action == ActionInvitationAccept {
		return Allow
	}

	if actor.PlatformAdmin {
		return Allow
	}

	if member == nil || !member.Active() {
		return Hidden
	}

	if member.Role == RoleAdmin {
		return Allow
	}

	if member.Role != RoleDeveloper {
		return Forbidden
	}

	switch action {
	case ActionApplicationRead:
		return Allow
	case ActionSecretRead, ActionSecretWrite, ActionSecretReveal,
		ActionTokenCreate, ActionTokenList, ActionTokenRevoke, ActionAuditRead:
		if owner == member.ID {
			return Allow
		}
	}

	return Forbidden
}
