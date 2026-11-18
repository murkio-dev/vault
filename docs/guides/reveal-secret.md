# Revelar um valor

Listagens mostram só metadados. A única forma (além do runtime) de ver o valor é **revelar**, e isso é auditado.

```sh
curl -s -X POST $VAULT/api/v1/applications/$APP_ID/members/me/secrets/$SECRET_ID/reveal \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

```json
{"data": {"id": "…", "name": "DATABASE_URL", "version": 2, "value": "postgres://..."}}
```

- Para uma versão antiga: `-d '{"version": 1}'` (corpo opcional; `0` ou ausente é a atual).
- Resposta sempre JSON e com `no-store`. Não guarde em cache nem em log.
- Cada revelação grava `secret.revealed` com o nome e a versão (nunca o valor). Se a auditoria falhar, o valor não é devolvido.
- Um developer revela só os próprios segredos. Um admin revela os de qualquer membro.
- O Caddy limita a rota a 30 por minuto por IP.

Para consumir valores no código, use um [token](create-token-and-read-env.md).

Referência: [Segredos](../api/secrets.md).
