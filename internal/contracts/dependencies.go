package contracts

import (
	auditcontracts "github.com/murkio-dev/vault/internal/restful/audit/contracts"
	membercontracts "github.com/murkio-dev/vault/internal/restful/member/contracts"
	secretcontracts "github.com/murkio-dev/vault/internal/restful/secret/contracts"
)

// Dependencies reúne o que mais de um service recebe do initializr raiz. Cada
// service monta o próprio repository com o db; aqui só entra o que atravessa
// services. Só o initializr raiz conhece as implementações.
type Dependencies struct {
	Guard   IAccessGuard
	Tx      IAuditedTx
	Values  ISecretValues
	Audit   auditcontracts.IAuditLogRepository
	Members membercontracts.IMemberRepository
	Secrets secretcontracts.ISecretRepository
}
