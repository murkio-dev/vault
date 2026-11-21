# Auditoria

Toda escrita grava uma linha em `audit_logs` **na mesma transação** da mudança. Se a linha não puder ser gravada, a operação desfaz tudo (fail-closed). Leituras sensíveis (`secret.revealed`, `env.read`) e acessos negados também entram.

## Listar

```http
GET /api/v1/applications/{app}/audit-logs?limit=50&action=secret.revealed&outcome=allowed
Authorization: Bearer <jwt>
```

```json
{
  "data": [
    {"id": "0194f3a2-...", "application_id": "…", "actor_type": "user", "actor_id": "…",
     "action": "secret.revealed", "outcome": "allowed", "target_type": "secret", "target_id": "…",
     "ip": "203.0.113.7", "user_agent": "curl/8.9", "request_id": "…",
     "metadata": {"name": "DATABASE_URL", "member_id": "…", "version": 2},
     "created_at": "2026-11-17T12:00:00Z"}
  ],
  "page": {"next_cursor": "0194f3a1-..."}
}
```

- Exige `audit.read`. **Admin** e platform admin veem tudo. **Developer** vê só as linhas em que ele é o ator (o filtro é forçado). Quem não é membro recebe `404`.
- Paginação por **cursor** ([Paginação](pagination.md)): `limit` (padrão 100, máximo 200) e `cursor`. Ordem por `id` decrescente (UUIDv7, que é a ordem do tempo).

### Filtros

| Query | Valor |
|---|---|
| `actor_id` | uuid |
| `outcome` | `allowed` ou `denied` |
| `action` | até 64 caracteres |
| `target_id` | até 64 caracteres |
| `from`, `to` | RFC 3339 |
| `cursor` | uuid (o `next_cursor` anterior) |

Valor inválido dá `400`.

### Campos

`actor_type` é `user` ou `token`. Textos são truncados em 256 caracteres. `metadata` guarda ids e nomes, **nunca valores de segredo** nem corpo de requisição.

## Ações gravadas

| Grupo | Ações |
|---|---|
| Aplicação | `application.created`, `application.updated`, `application.deleted` |
| Membro | `member.updated`, `member.removed` |
| Convite | `invitation.created`, `invitation.accepted`, `invitation.revoked` |
| Segredo | `secret.created`, `secret.updated`, `secret.renamed`, `secret.deleted`, `secret.revealed` |
| Chave de API | `api_key.created`, `api_key.updated`, `api_key.rotated`, `api_key.revoked` (as linhas anteriores à migration `0009` seguem como `token.created` e `token.revoked`, porque a trilha é imutável) |
| Runtime | `env.read` |
| Acesso | `access.denied` |

`PUT` e `PATCH` de aplicação gravam `application.updated`. A constante `member.added` existe no código, mas nenhuma operação a grava hoje: entrar por convite grava `invitation.accepted`.

`access.denied` (com `outcome: denied`) é gravado fora da transação, e uma falha ao gravá-lo não muda a resposta.

## Imutável

Um trigger recusa qualquer `UPDATE`, e outro recusa `DELETE` de linhas com menos de um ano. Não existe rota de escrita. Veja [Retenção](../operations/retention.md).

Arquivo de teste: [`hacks/http/audit-logs/list.http`](../../hacks/http/audit-logs/list.http).
