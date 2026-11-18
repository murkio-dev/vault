# Revogar um token

```sh
# ache o id do token
curl -s $VAULT/api/v1/applications/$APP_ID/members/me/tokens -H "Authorization: Bearer $ACCESS_TOKEN"

# revogue
curl -s -X POST $VAULT/api/v1/applications/$APP_ID/members/me/tokens/$TOKEN_ID/revoke \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

- Resposta `204`. A revogação vale **na hora**: o próximo `runtime/env` com esse token dá `401`.
- Revogar duas vezes é seguro e não duplica a auditoria.
- Um admin revoga os tokens de qualquer membro (troque `me` pelo id do membro). Um developer, só os próprios.
- Token vazado? Revogue, crie outro e troque os segredos que ele podia ler.

Revogar o acesso de uma pessoa inteira: remova (`DELETE …/members/{member}`) ou suspenda (`PATCH` com `"status": "suspended"`) o membro. Os tokens dele param na hora.

Referência: [Tokens](../api/tokens.md).
