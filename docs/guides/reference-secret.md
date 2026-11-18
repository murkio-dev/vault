# Referenciar um segredo

Uma referência faz um segredo apontar para o valor de outro, sem copiar. Útil quando dois membros usam a mesma `DATABASE_URL`.

```sh
curl -s -X POST $VAULT/api/v1/applications/$APP_ID/members/me/secrets \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "DATABASE_URL", "type": "database_uri", "source": "reference", "reference_secret_id": "<id do alvo>"}'
```

Regras:

- sem `value`;
- o alvo é um segredo `static` vivo, da **mesma aplicação** e do **mesmo tipo**;
- você precisa de permissão de revelar o segredo do dono do alvo (um developer só referencia os próprios, um admin referencia qualquer um);
- referência de referência é recusada;
- um alvo inexistente e um alvo sem permissão dão o mesmo `400` em `reference_secret_id`.

Ao ler (`reveal` ou `runtime/env`), o valor é o **atual** do alvo, e o `version` da referência é `0`. Se o alvo for trocado, a referência acompanha. Não dá para apagar um segredo enquanto houver referência para ele (`409`).

Referência: [Segredos](../api/secrets.md).
