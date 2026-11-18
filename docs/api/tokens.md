# Tokens

Credenciais do SDK. `/api/v1/applications/{app}/members/{member}/tokens`. Credencial: JWT. `{member}` é o id ou `me`.

Um admin gerencia os tokens de qualquer membro. Um developer, só os próprios.

## Criar

```http
POST /api/v1/applications/{app}/members/me/tokens
Content-Type: application/json

{"name": "notebook", "expires_at": "2027-01-01T00:00:00Z"}
```

```json
{"data": {"id": "…", "application_id": "…", "member_id": "…", "uri_id": "3f9c0a7e1b2d4c5e8a6b7c8d",
          "name": "notebook", "prefix": "sk_vt_Zk3Abc", "expires_at": "2027-01-01T00:00:00Z",
          "last_used_at": null, "revoked_at": null, "created_at": "2026-11-17T12:00:00Z",
          "token": "sk_vt_Zk3AbcDEF...", "uri": "https://api.murkio.com/v1/vault/3f9c0a7e1b2d4c5e8a6b7c8d"}}
```

- Exige `token.create`. Rota **só-JSON**, com corpo estrito.
- `name`: 1 a 100 caracteres, sem caracteres de controle.
- `expires_at` é opcional (RFC 3339), precisa estar no futuro e a no máximo 10 anos.
- O `token` (`sk_vt_` + 43 caracteres, de 32 bytes aleatórios) **aparece só aqui**. No banco fica o SHA-256 hex. Copie na hora.
- `prefix` (`sk_vt_` + 6 caracteres) existe para você reconhecer o token numa lista.
- `uri` é `APPLICATION_BACKEND` + `/` + `uri_id`: um identificador, não uma rota. O SDK chama `/runtime/{uri_id}/env`.
- Membro suspenso dá `409` (`Member suspended`). Audita `token.created` (`name`, `prefix`, `member_id`).

## Listar

`GET …/tokens`. Exige `token.list`. Até 200 tokens, **incluindo os revogados**, do mais novo ao mais antigo. Nunca traz o token nem o hash.

## Revogar

`POST …/tokens/{id}/revoke`. Exige `token.revoke` e responde `204`.

- Vale **na hora**: o token é conferido no banco a cada requisição.
- É idempotente: revogar um token já revogado responde `204` e não grava segunda auditoria.
- Token de outro membro (sem ser admin) dá `404`. Audita `token.revoked` (`name`, `prefix`, `member_id`).

## Quando um token deixa de valer

- foi revogado;
- passou do `expires_at`;
- o membro foi suspenso ou removido;
- a aplicação foi apagada.

Arquivos de teste: [`hacks/http/tokens/`](../../hacks/http/tokens).
