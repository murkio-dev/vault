package audited

import (
	"context"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/sqlite"
)

type Audited struct {
	Transactor sqlite.Transactor
	Recorder   contracts.IAuditLogCreator
}

func New(transactor sqlite.Transactor, recorder contracts.IAuditLogCreator) Audited {
	return Audited{Transactor: transactor, Recorder: recorder}
}

var _ contracts.IAuditedTx = Audited{}

// Run executa fn e, só se ela der certo, grava entry na trilha. As duas coisas
// estão na mesma transação: erro em qualquer uma desfaz as duas. Isso torna a
// auditoria fail-closed, porque uma mudança sem linha de trilha não existe, e
// uma linha de trilha de mudança que falhou também não. A auditoria vem depois
// da mudança para o entry poder descrever o que realmente aconteceu.
// Exemplo: criar um segredo e a linha secret.created entram ou saem juntos.
func (a Audited) Run(ctx context.Context, entry entities.AuditLog, fn func(ctx context.Context) error) error {
	return a.Transactor.Do(ctx, func(ctx context.Context) error {
		if err := fn(ctx); err != nil {
			return err
		}
		_, err := a.Recorder.Create(ctx, entry)
		return err
	})
}
