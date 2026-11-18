# Ler a trilha de auditoria

```sh
curl -s "$VAULT/api/v1/applications/$APP_ID/audit-logs?limit=50" -H "Authorization: Bearer $ACCESS_TOKEN"
```

Perguntas comuns:

| Pergunta | Query |
|---|---|
| Quem revelou segredos? | `?action=secret.revealed` |
| Houve acesso negado? | `?outcome=denied` |
| O que a pessoa X fez? | `?actor_id=<uuid>` |
| O que aconteceu em um segredo? | `?target_id=<id do segredo>` |
| O que aconteceu em um período? | `?from=2026-11-01T00:00:00Z&to=2026-11-17T00:00:00Z` |

Paginação por cursor: repita a chamada com `cursor=<page.next_cursor>` até `next_cursor` vir `null`.

Admin vê tudo. Developer vê só as próprias linhas.

Cada linha traz `actor_type`, `actor_id`, `token_id` (quando foi o SDK), `ip`, `user_agent`, `request_id` e `metadata` (ids e nomes, nunca valores).

Referência: [Auditoria](../api/audit-logs.md).
