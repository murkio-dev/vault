# Aplicações

`/api/v1/applications`. Credencial: JWT. Aceita [lote](batch.md) em criar, atualizar e apagar.

## Criar

```http
POST /api/v1/applications
Authorization: Bearer <jwt>
Content-Type: application/json

{"name": "loja-api"}
```

```json
{"data": {"id": "0194f3a2-...", "name": "loja-api", "created_by": "…", "updated_at": null, "created_at": "2026-11-17T12:00:00Z"}}
```

- Um objeto ou uma lista (`[{"name": "a-1"}, {"name": "b-2"}]`). `created_by` vem sempre do JWT, nunca do corpo.
- `name`: 3 a 50 caracteres, `^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$`.
- Quem cria vira **admin** ativo da aplicação, na mesma transação. Audita `application.created` (`name`, `admin_member_id`).
- `400` nome inválido ou corpo vazio. `409` já existe uma aplicação **ativa** com esse nome (o nome é único entre todos os tenants). `207` em lote misto.

## Listar

`GET /api/v1/applications?page=1&limit=50`

Só aceita JWT de pessoa. Platform admin vê todas as aplicações ativas. Os demais veem só aquelas em que são membros ativos. Ordem por `id`. Resposta com `data` e `page` ([paginação](pagination.md)).

## Buscar uma

`GET /api/v1/applications/{id}`. Exige `application.read` (admin, developer ou platform admin). Quem não é membro recebe `404`. `400` se o id não for um uuid.

## Atualizar

| Método | Efeito |
|---|---|
| `PUT /applications[/{id}]` | substitui (o `name` é obrigatório e validado) |
| `PATCH /applications[/{id}]` | parcial (um `name` vazio mantém o atual) |

```http
PATCH /api/v1/applications/0194f3a2-...
Content-Type: application/json

{"name": "loja-api-v2"}
```

- Com `{id}`: o id do path vence o do corpo, e o corpo com mais de um item dá `400`.
- Sem `{id}`: cada item do corpo traz o próprio `id` (uuid zero dá `400` para o item).
- Exige `application.update` (admin ou platform admin). `404` se não existe, `409` se o nome já está em uso.
- Audita `application.updated` (`name`), tanto no `PUT` quanto no `PATCH`.

## Apagar

```http
DELETE /api/v1/applications
Content-Type: application/json

["0194f3a2-...", {"id": "0194f3a3-..."}]
```

Ou `DELETE /api/v1/applications/{id}`. Exige `application.delete`. Responde `204`, ou `207` quando parte falha.

É um **soft delete**: marca `deleted_at` na aplicação, revoga os tokens, marca segredos e membros como apagados e revoga os convites pendentes. As versões cifradas e a auditoria ficam. O nome volta a ficar livre.

Audita `application.deleted`.

Arquivos de teste: [`hacks/http/applications/`](../../hacks/http/applications).
