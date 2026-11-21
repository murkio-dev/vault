# Quickstart

Do zero até ler uma variável com o token do SDK. Os exemplos usam `curl` e o Vault local em `http://localhost:8083`.

Você precisa do JWT da sua conta Murkio (emitido pelo Identity):

```sh
export VAULT=http://localhost:8083
export ACCESS_TOKEN=<jwt da conta Murkio>
```

## 1. Crie a aplicação

```sh
curl -s -X POST $VAULT/api/v1/applications \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "minha-api"}'
```

Guarde o `data.id` como `APP_ID`. Quem cria vira `admin`.

## 2. Descubra o seu `uri_id`

```sh
curl -s $VAULT/api/v1/applications/$APP_ID/members/me \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

Guarde o `data.uri_id`.

## 3. Guarde um segredo

```sh
curl -s -X POST $VAULT/api/v1/applications/$APP_ID/members/me/secrets \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "DATABASE_URL", "type": "database_uri", "value": "postgres://app:senha@db.internal:5432/app"}'
```

A resposta traz só metadados (`id`, `version`, ...), nunca o valor.

## 4. Crie um token

```sh
curl -s -X POST "$VAULT/api/v1/api-keys?application_id=$APP_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "notebook", "type": "secret"}'
```

Copie `data.key` (`sk_vt_...`) agora: ele **não aparece de novo**.

## 5. Leia o ambiente como o SDK

```sh
curl -s $VAULT/api/v1/runtime/$URI_ID/env \
  -H "Authorization: Bearer sk_vt_..."
```

```json
{"data":{"uri_id":"...","variables":[{"name":"DATABASE_URL","type":"database_uri","version":1,"value":"postgres://..."}]}}
```

Use `?names=A,B` para pedir só algumas chaves.

## 6. Veja a auditoria

```sh
curl -s "$VAULT/api/v1/applications/$APP_ID/audit-logs?limit=20" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

Aparecem `application.created`, `secret.created`, `api_key.created` e `env.read`.

Mais: [Guias](../README.md#guias) e [Arquivos `.http`](../development/http-files.md).
