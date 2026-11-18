package auditcontracts

import (
	"context"
	"time"

	"github.com/murkio-dev/vault/internal/entities"
)

type IAuditLogRepository interface {
	Create(ctx context.Context, log entities.AuditLog) (entities.AuditLog, error)
	// List lê a trilha do mais novo para o mais antigo, com cursor em
	// filter.Cursor. filter.ApplicationID é obrigatório.
	List(ctx context.Context, filter entities.AuditFilter, limit int) ([]entities.AuditLog, error)
	// PurgeExpired apaga da trilha o que passou da retenção. PurgeExpired apaga
	// até limit linhas de até agora - AuditRetention - margem (now vem de quem
	// chama, para o teste e o job controlarem o relógio) e devolve quantas apagou;
	// menos que limit diz que não sobrou nada vencido. É a única escrita que a
	// trilha aceita além do Create, e o trigger do banco recusa qualquer linha
	// dentro da retenção, então um engano aqui nunca apaga auditoria no prazo.
	PurgeExpired(ctx context.Context, now time.Time, limit int) (deleted int64, err error)
}
