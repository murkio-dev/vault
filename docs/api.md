# API HTTP

O contrato completo da API REST do Vault: rotas, permissões e erros. Cada recurso tem uma página com corpos e exemplos.

- Prefixo: `/api/v1` (atrás do Caddy: `https://api.murkio.com/v1/vault/...`).
- Autenticação: `Authorization: Bearer <credencial>`. Veja [Autenticação](api/authentication.md).
- Toda resposta de `/api/*` sai com `Cache-Control: no-store`.
- Corpo de requisição: no máximo 64 KiB.
- Cada rota existe com e sem barra final (`/secrets` e `/secrets/`).

```http
GET /api/v1/applications/3a1f.../members/me HTTP/1.1
Authorization: Bearer eyJhbGciOiJFZERTQSIs...
Accept: application/json
```

## Rotas

`{app}` é o id da aplicação, `{member}` é o id do membro (ou `me`, onde indicado) e `{id}` é o id do recurso. Credencial: **JWT** é a pessoa; **SDK** é o token `sk_vt_`.

### Aplicações

| Método e caminho | Ação (permissão) | Credencial | Sucesso | Página |
|---|---|---|---|---|
| `POST /applications` | qualquer pessoa autenticada | JWT | `201` (ou `207`) | [Aplicações](api/applications.md) |
| `GET /applications` | lista as da pessoa | JWT | `200` | |
| `GET /applications/{id}` | `application.read` | JWT | `200` | |
| `PUT /applications[/{id}]` | `application.update` | JWT | `200` (ou `207`) | |
| `PATCH /applications[/{id}]` | `application.update` | JWT | `200` (ou `207`) | |
| `DELETE /applications[/{id}]` | `application.delete` | JWT | `204` (ou `207`) | |

### Membros

| Método e caminho | Ação | Credencial | Sucesso | Página |
|---|---|---|---|---|
| `GET /applications/{app}/members` | `member.list` | JWT | `200` | [Membros](api/members.md) |
| `GET /applications/{app}/members/me` | `application.read` | JWT | `200` | |
| `PATCH /applications/{app}/members/{member}` | `member.update` | JWT | `200` | |
| `DELETE /applications/{app}/members/{member}` | `member.remove` | JWT | `204` | |

### Convites

| Método e caminho | Ação | Credencial | Sucesso | Página |
|---|---|---|---|---|
| `POST /applications/{app}/invitations` | `invitation.manage` | JWT | `201` | [Convites](api/invitations.md) |
| `GET /applications/{app}/invitations` | `invitation.manage` | JWT | `200` | |
| `DELETE /applications/{app}/invitations/{id}` | `invitation.manage` | JWT | `204` | |
| `POST /invitations/accept` | `invitation.accept` | JWT | `201` | |

### Segredos (`/applications/{app}/members/{member}/secrets`)

| Método e caminho | Ação | Credencial | Sucesso | Página |
|---|---|---|---|---|
| `POST …/secrets` | `secret.write` | JWT | `201` (ou `207`) | [Segredos](api/secrets.md) |
| `GET …/secrets` | `secret.read` | JWT | `200` | |
| `GET …/secrets/{id}` | `secret.read` | JWT | `200` | |
| `PUT …/secrets/{id}` | `secret.write` | JWT | `200` | |
| `PATCH …/secrets/{id}` | `secret.write` | JWT | `200` | |
| `DELETE …/secrets/{id}` | `secret.write` | JWT | `204` | |
| `GET …/secrets/{id}/versions` | `secret.read` | JWT | `200` | |
| `POST …/secrets/{id}/reveal` | `secret.reveal` | JWT | `200` | |

### Tokens (`/applications/{app}/members/{member}/tokens`)

| Método e caminho | Ação | Credencial | Sucesso | Página |
|---|---|---|---|---|
| `POST …/tokens` | `token.create` | JWT | `201` | [Tokens](api/tokens.md) |
| `GET …/tokens` | `token.list` | JWT | `200` | |
| `POST …/tokens/{id}/revoke` | `token.revoke` | JWT | `204` | |

### Runtime

| Método e caminho | Ação | Credencial | Sucesso | Página |
|---|---|---|---|---|
| `GET /runtime/{uri_id}/env` | `env.read` | **SDK** | `200` | [Runtime](api/runtime.md) |

### Auditoria

| Método e caminho | Ação | Credencial | Sucesso | Página |
|---|---|---|---|---|
| `GET /applications/{app}/audit-logs` | `audit.read` | JWT | `200` | [Auditoria](api/audit-logs.md) |

## Matriz de permissões

Quem decide é `entities.Decide` (função pura, testada em matriz). Nenhum use case compara papel por conta própria.

| Ação | Platform admin | Admin | Developer | SDK |
|---|---|---|---|---|
| `application.create`, `invitation.accept` | sim | sim | sim | não |
| `application.read` | sim | sim | sim | não |
| `application.update`, `application.delete` | sim | sim | `403` | não |
| `member.list`, `member.update`, `member.remove` | sim | sim | `403` | não |
| `invitation.manage` | sim | sim | `403` | não |
| `secret.read`, `secret.write`, `secret.reveal` | sim | sim (de todos) | só os próprios | não |
| `token.create`, `token.list`, `token.revoke` | sim | sim (de todos) | só os próprios | não |
| `audit.read` | sim | sim (tudo) | só as próprias linhas | não |
| `env.read` | não | não | não | só o próprio membro |

Quem não é membro (ou está suspenso, ou foi removido) recebe `404`, igual a um recurso que não existe. Detalhes em [Permissões](security/permissions.md).

## Erros

Formato [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) (`type`, `title`, `status`, `detail`, `instance`, `invalid_params` com `field`, `reason` e `value`). Um erro sai como `{"error": {...}}`, dois ou mais como `{"errors": [...]}`.

| Status | `type` (sufixo) | Quando |
|---|---|---|
| `400` | `#validation` | corpo, path ou query inválidos |
| `401` | `#unauthorized` | credencial ausente, inválida, revogada ou do tipo errado |
| `403` | `#forbidden` | é membro, mas o papel não permite |
| `404` | `#not-found` | não existe, ou o chamador não pode saber que existe |
| `409` | `#conflict` | nome repetido, último admin, já é membro, segredo referenciado |
| `429` | | limite de taxa do **Caddy** (o Vault não limita taxa) |
| `500` | `#internal` | erro interno (a mensagem nunca vaza) |
| `503` | `#unavailable` | dependência indisponível |

O prefixo do `type` é `https://murkio.com/en-us/docs/vault/errors/`. Mais em [Erros](api/errors.md).

Resposta com sucesso e erro no mesmo lote sai como `207 Multi-Status`. Veja [Lote](api/batch.md).
