# Revogar um token

```sh
# ache o id do token
curl -s "$VAULT/api/v1/api-keys?application_id=$APP_ID" -H "Authorization: Bearer $ACCESS_TOKEN"

# revogue
curl -s -X DELETE "$VAULT/api/v1/api-keys/$TOKEN_ID?application_id=$APP_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

- Resposta `204`. A revogação vale **na hora**: o próximo `runtime/env` com esse token dá `401`.
- Revogar duas vezes é seguro e não duplica a auditoria.
- Um admin revoga os tokens de qualquer membro (passe `member_id` com o id do membro). Um developer, só os próprios.
- Token vazado? Revogue, crie outro e troque os segredos que ele podia ler.
- Troca planejada, sem vazamento? Use `POST /api/v1/api-keys/{id}/rotate`: a antiga vale por mais 72h.

Revogar o acesso de uma pessoa inteira: remova (`DELETE …/members/{member}`) ou suspenda (`PATCH` com `"status": "suspended"`) o membro. Os tokens dele param na hora.

Referência: [Chaves de API](../api/api-keys.md).
