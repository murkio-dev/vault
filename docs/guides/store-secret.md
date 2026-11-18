# Guardar um segredo

```sh
curl -s -X POST $VAULT/api/v1/applications/$APP_ID/members/me/secrets \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "DATABASE_URL", "type": "database_uri", "value": "postgres://app:senha@db.internal:5432/app"}'
```

1. Nome em maiúsculas com `_` e dígitos (`DATABASE_URL`), até 128 caracteres.
2. Escolha o tipo: `string`, `password`, `url`, `database_uri`, `api_key`, `jwt_key` ou `paseto_key`.
3. O valor tem até 32 KiB. Para `url` e `database_uri` precisa de esquema e host.
4. A resposta traz `version: 1` e **não** traz o valor.

Vários de uma vez (até 50): mande uma lista no mesmo `POST`.

Para trocar o valor: `PUT …/secrets/{id}` com `{"value": "..."}` cria a versão 2 e guarda a 1. Para renomear: `PATCH …/secrets/{id}` com `{"name": "..."}`.

Cada tabela guarda até 100 segredos.

Referência: [Segredos](../api/secrets.md).
