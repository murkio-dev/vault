# Criar um token e ler o ambiente

O SDK (ou o seu código) não usa o JWT: usa um token `sk_vt_...`, preso ao seu membro.

## 1. Descubra o seu `uri_id`

```sh
curl -s $VAULT/api/v1/applications/$APP_ID/members/me -H "Authorization: Bearer $ACCESS_TOKEN"
```

## 2. Crie o token

```sh
curl -s -X POST $VAULT/api/v1/applications/$APP_ID/members/me/tokens \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "notebook", "expires_at": "2027-01-01T00:00:00Z"}'
```

Copie o `data.token`. **Ele não aparece de novo.** Sem `expires_at` o token não expira (o máximo, quando informado, é 10 anos).

## 3. Leia o ambiente

```sh
export VAULT_TOKEN=sk_vt_...
curl -s "$VAULT/api/v1/runtime/$URI_ID/env" -H "Authorization: Bearer $VAULT_TOKEN"
curl -s "$VAULT/api/v1/runtime/$URI_ID/env?names=DATABASE_URL" -H "Authorization: Bearer $VAULT_TOKEN"
```

Transformar a resposta em variáveis de ambiente:

```sh
eval "$(curl -s "$VAULT/api/v1/runtime/$URI_ID/env" -H "Authorization: Bearer $VAULT_TOKEN" \
  | jq -r '.data.variables[] | "export \(.name)=\(.value | @sh)"')"
```

O token só lê o **seu** `uri_id` (outro dá `404` e fica registrado como `access.denied`). Guarde o token como um segredo: em um cofre de CI, nunca no repositório.

Referências: [Tokens](../api/tokens.md) e [Runtime](../api/runtime.md).
