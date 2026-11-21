# Chaves de API

Credenciais do SDK. `/api/v1/api-keys`, a mesma rota de chaves do Identity e do Messaging. Credencial: JWT.

De quem são as chaves vem na query, em toda rota:

- `application_id` (obrigatório): a aplicação.
- `member_id` (opcional): o id do membro ou `me`. Ausente vale `me`.

A chave é presa a um **membro** e herda a permissão dele. Um admin gerencia as chaves de qualquer membro. Um developer, só as próprias.

## Tipos

| Tipo | Prefixo | Para que serve |
|---|---|---|
| `secret` | `sk_vt_` | Autentica o SDK no runtime e lê o ambiente do membro. |
| `publishable` | `pk_vt_` | Só identifica a aplicação. **Nunca** autentica e nunca lê, lista ou revela segredo: qualquer rota responde `401`. |

## Criar

```http
POST /api/v1/api-keys?application_id={app}&member_id=me
Content-Type: application/json

{"name": "notebook", "type": "secret", "expires_at": "2027-01-01T00:00:00Z"}
```

```json
{"data": {"id": "…", "application_id": "…", "member_id": "…", "uri_id": "3f9c0a7e1b2d4c5e8a6b7c8d",
          "name": "notebook", "type": "secret", "prefix": "sk_vt_Zk3Abc", "last_used_at": null,
          "expires_at": "2027-01-01T00:00:00Z", "updated_at": null, "created_at": "2026-11-17T12:00:00Z",
          "deleted_at": null,
          "key": "sk_vt_Zk3AbcDEF...", "uri": "https://api.murkio.com/v1/vault/3f9c0a7e1b2d4c5e8a6b7c8d"}}
```

- Exige `api_key.create`. Rota **só-JSON**, com corpo estrito.
- `name`: 1 a 100 caracteres, sem caracteres de controle.
- `type`: `secret` ou `publishable` (obrigatório).
- `expires_at` é opcional (RFC 3339), precisa estar no futuro e a no máximo 10 anos.
- A `key` (prefixo + 43 caracteres, de 32 bytes aleatórios) **aparece só aqui**. No banco fica o SHA-256 hex. Copie na hora.
- `prefix` (prefixo + 6 caracteres) existe para você reconhecer a chave numa lista.
- `uri` é `APPLICATION_BACKEND` + `/` + `uri_id`: um identificador, não uma rota. O SDK chama `/runtime/{uri_id}/env`.
- Membro suspenso dá `409` (`Member suspended`). Audita `api_key.created` (`name`, `type`, `prefix`, `member_id`).

## Listar e ler

- `GET /api/v1/api-keys`: exige `api_key.read`. Até 200 chaves, **incluindo as apagadas**, da mais nova à mais antiga.
- `GET /api/v1/api-keys/{id}`: exige `api_key.read`. Devolve a chave mesmo apagada.

Nenhuma das duas traz a chave nem o hash.

## Renomear

`PATCH /api/v1/api-keys/{id}` e `PUT /api/v1/api-keys/{id}` com `{"name": "deploy"}`. Exige `api_key.update`.

- O nome é a única coisa que muda numa chave.
- No `PATCH`, nome ausente mantém o atual. No `PUT`, ele é obrigatório.
- Chave apagada dá `404`. Audita `api_key.updated`.

## Rotacionar

`POST /api/v1/api-keys/{id}/rotate`. Exige `api_key.create` e responde `201` com a chave nova (só-JSON).

- A nova tem o mesmo nome e tipo. A `key` dela aparece só nesta resposta.
- A antiga continua valendo por **72 horas** (ou até o vencimento dela, se for antes), para dar tempo de trocar a chave onde ela está configurada.
- Chave apagada dá `404`, membro suspenso dá `409`. Audita `api_key.rotated` (ids e prefixos, nunca a chave).

## Apagar

`DELETE /api/v1/api-keys/{id}`. Exige `api_key.delete` e responde `204`.

- Vale **na hora**: a chave é conferida no banco a cada requisição.
- É idempotente: apagar uma chave já apagada responde `204` e não grava segunda auditoria.
- Chave de outro membro (sem ser admin) dá `404`. Audita `api_key.revoked` (`name`, `type`, `prefix`, `member_id`).

## Quando uma chave deixa de valer

- foi apagada;
- passou do `expires_at` (inclusive o prazo de 72h de uma rotação);
- o membro foi suspenso ou removido;
- a aplicação foi apagada.

Os tokens `sk_vt_` criados antes da migration `0009` viraram chaves `secret` e continuam valendo.

Arquivos de teste: [`hacks/http/api-keys/`](../../hacks/http/api-keys).
