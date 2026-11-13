# Retenção

O que o vault apaga por prazo, e o que ainda depende do job de expurgo.

## Auditoria (`audit_logs`): 1 ano

- O banco decide, não o código. O trigger `audit_logs_no_delete` (migration `0007`) recusa o `DELETE` de qualquer linha com menos de 1 ano e deixa passar só o que já venceu. O trigger `audit_logs_no_update` recusa o `UPDATE` sempre. Um erro no serviço ou no job nunca apaga auditoria dentro do prazo.
- A comparação do trigger é de texto, por segundo, em UTC (o driver grava `created_at` em UTC).
- O expurgo é `AuditRepository.PurgeExpired(ctx, now, limit)` (contrato `IAuditLogPurger`). Apaga as linhas mais antigas que `now - 1 ano - 1 h`, no máximo `limit` por chamada (1 a 5000). A hora de folga evita cair no limite do trigger.
- Lotes pequenos seguram o lock de escrita do SQLite por pouco tempo: a API segue gravando entre um lote e outro. Um lote que volta com menos linhas que `limit` diz que não sobrou nada vencido.
- Leitura continua pela API (`GET .../audit-logs`) e só enxerga o que ainda está no banco: a trilha não é um total desde o começo, é a dos últimos 12 meses.

## O que falta: o job

Nada chama o `PurgeExpired` ainda. O vault é um binário só e não tem workers; o expurgo entra junto com os jobs, que só começam depois das APIs (ver `../tasks.md`). Quando existir, o job deve:

1. Rodar a cada hora, com `now = time.Now()`.
2. Chamar `PurgeExpired(ctx, now, 1000)` em laço até vir menos de 1000, com no máximo 5 minutos por rodada. O que sobrar fica para a próxima.
3. Logar só a contagem apagada. Nunca o conteúdo.

Até lá, a auditoria só cresce. Para apagar à mão o que passou de 1 ano, o mesmo `DELETE` funciona direto no SQLite (o trigger protege o resto).

## Outras tabelas

Não há retenção definida para as demais, e nenhuma é apagada hoje:

| Tabela | Situação |
| --- | --- |
| `outbox_events` | linhas publicadas ficam (o payload de um comando já é trocado por `{}` ao publicar). O relay que publica não existe ainda; a retenção das publicadas entra com ele |
| `invitations` | convite vencido ou aceito fica; guarda hash, não o token |
| `access_tokens` | revogado ou vencido fica, para a auditoria apontar para ele |
| `secret_versions` | nunca apagada por prazo: guarda o histórico cifrado para rollback |

Decidir prazo para `outbox_events` publicadas, convites vencidos e tokens revogados é trabalho do job, e precisa de decisão de produto (por quanto tempo o histórico de convites e tokens interessa).
