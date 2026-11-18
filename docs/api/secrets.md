# Segredos

`/api/v1/applications/{app}/members/{member}/secrets`. Credencial: JWT. `{member}` é o id do membro ou `me`.

Um admin acessa a tabela de qualquer membro. Um developer, só a própria (`403`/`404` nas dos outros).

## Criar

```http
POST /api/v1/applications/{app}/members/me/secrets
Content-Type: application/json

{"name": "DATABASE_URL", "type": "database_uri", "value": "postgres://app:senha@db.internal:5432/app"}
```

```json
{"data": {"id": "…", "application_id": "…", "member_id": "…", "name": "DATABASE_URL", "type": "database_uri",
          "source": "static", "reference_secret_id": null, "version": 1, "created_by": "…",
          "created_at": "2026-11-17T12:00:00Z", "updated_at": null}}
```

A resposta **nunca** traz o valor.

- Um objeto ou uma lista de até **50** itens ([lote](batch.md)). Exige `secret.write`. Corpo só-JSON e estrito.
- `name`: `^[A-Z][A-Z0-9_]{0,127}$`, único por membro entre os segredos vivos (`409`).
- `type`: `string`, `password`, `url`, `database_uri`, `api_key`, `jwt_key`, `paseto_key`.
- `source`: `static` (padrão) ou `reference`. `generated` e `dynamic` dão `400`.
- `value`: não vazio, até 32 KiB, UTF-8 válido, sem byte NUL. Para `url` e `database_uri` precisa de esquema e host e sem `CR`/`LF`. O erro nunca repete o valor.
- Uma tabela guarda no máximo **100** segredos (`409`).
- Audita `secret.created` (`name`, `type`, `source`, `member_id`).

### Referência

```json
{"name": "REPLICA_URL", "type": "database_uri", "source": "reference", "reference_secret_id": "<id do segredo alvo>"}
```

- Sem `value`. O alvo precisa ser um segredo `static` vivo, da **mesma aplicação** e do **mesmo tipo**.
- Quem cria precisa de `secret.reveal` sobre o dono do alvo. Referência de referência é recusada.
- Alvo ruim dá `400` em `reference_secret_id`, com o mesmo texto para "não existe" e "sem permissão".
- Uma referência tem `version: 0` e devolve o valor **atual** do alvo.
- Não dá para apagar um segredo que ainda é alvo de uma referência (`409`).

## Listar

`GET …/secrets`. Exige `secret.read`. Só metadados (nome, tipo, origem, versão, datas), sem paginação.

## Buscar um

`GET …/secrets/{id}`. Exige `secret.read`.

## Nova versão

```http
PUT …/secrets/{id}
{"value": "postgres://app:nova@db.internal:5432/app"}
```

Cria a versão N+1 (as anteriores ficam cifradas e imutáveis). Só em segredo `static` (em referência dá `400`). Exige `secret.write`. Audita `secret.updated` (`name`, `version`, `member_id`). Duas escritas concorrentes: a que perde recebe `404`.

## Renomear

```http
PATCH …/secrets/{id}
{"name": "DB_URL"}
```

Audita `secret.renamed` (`from`, `to`).

## Apagar

`DELETE …/secrets/{id}` responde `204`. É soft delete e as versões ficam. Audita `secret.deleted`.

## Versões

`GET …/secrets/{id}/versions`. Exige `secret.read`. Até 500, da mais nova à mais antiga, com `version`, `created_by` e `created_at` (sem valor).

## Revelar

```http
POST …/secrets/{id}/reveal
{"version": 2}
```

```json
{"data": {"id": "…", "name": "DATABASE_URL", "version": 2, "value": "postgres://..."}}
```

- Exige `secret.reveal`. Resposta **sempre JSON**, qualquer que seja o `Accept`. O corpo é opcional: `version` `0` ou ausente é a atual, e uma versão que não existe dá `404`.
- Decifra e audita dentro da mesma transação: se a auditoria falhar, o valor **não** é devolvido. Audita `secret.revealed` (`name`, `member_id`, `version`, e `resolved_from` em referências).
- Em referência, o `version` pedido é ignorado e vale o valor atual do alvo.

## Permissões

| Ação | Admin | Developer |
|---|---|---|
| `secret.read`, `secret.write`, `secret.reveal` | em qualquer membro | só `me` |

`PUT`, `PATCH` e `DELETE` existem só com `{id}`. O lote vale apenas no `POST`.

Arquivos de teste: [`hacks/http/secrets/`](../../hacks/http/secrets).
