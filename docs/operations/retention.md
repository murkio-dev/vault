# Retenção

O que o Vault guarda, por quanto tempo e o que apaga.

| Dado | Retenção |
|---|---|
| Segredos e membros apagados | soft delete: a linha fica com `deleted_at` |
| Versões de segredo | para sempre (o banco recusa apagar), mesmo com o segredo apagado |
| Convites | ficam; o status (`expired`, `revoked`, `accepted`) é calculado |
| Tokens | ficam, inclusive os revogados e os expirados |
| Auditoria | **1 ano** |
| Outbox (`outbox_events`) | as linhas publicadas ficam; o `payload` é esvaziado (`{}`) ao publicar |

## Auditoria

`audit_logs` é append-only. Um trigger recusa `UPDATE` sempre e `DELETE` de linhas com menos de um ano. O expurgo existe no repository (`PurgeExpired(ctx, now, limit)`): apaga linhas mais antigas que `now - 1 ano - 1 h` (a hora de folga evita cair no limite do trigger), com `limit` de 1 a **5000** por chamada. Lotes pequenos seguram o lock de escrita do SQLite por pouco tempo, e uma chamada que volta com menos de `limit` linhas diz que não sobrou nada vencido. **Nenhuma rota chama esse método hoje**: ele é para um job de retenção, que ainda não existe. Enquanto isso, a tabela só cresce. Para apagar à mão o que passou de um ano, o mesmo `DELETE` funciona direto no SQLite (o trigger protege o resto).

A leitura pela API só enxerga o que ainda está no banco: a trilha não é o total desde o começo, é a dos últimos 12 meses depois que o job existir.

Desenho sugerido para o job: rodar a cada hora, chamar `PurgeExpired(ctx, now, 1000)` em laço até vir menos de 1000 (no máximo 5 minutos por rodada) e logar só a contagem apagada, nunca o conteúdo.

## Outbox

Criar um convite grava um evento `email.send` na tabela `outbox_events` (exchange `messaging.commands`, template `vault_invitation`, dados `{token, role}`) **na mesma transação** do convite. A publicação para o Messaging é de um worker externo, que ainda não existe: até lá a resposta da criação devolve o `token` do convite ao admin, e o evento fica pendente (`published_at` nulo). Depois de publicado, o `payload` é substituído por `{}`, porque contém o token.

## Apagar uma aplicação ou um membro

Não apaga linhas: marca `deleted_at`, revoga tokens e convites pendentes e libera os nomes. As versões cifradas e a auditoria permanecem.

## Decisões em aberto

Não há prazo definido para `outbox_events` publicadas, convites vencidos e tokens revogados: nada disso é apagado hoje. Definir quanto tempo esse histórico interessa é decisão de produto, e a execução é do job.
