# Criar uma aplicação

Uma aplicação é o espaço onde ficam membros, segredos e tokens. Quem a cria vira admin.

```sh
curl -s -X POST $VAULT/api/v1/applications \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "loja-api"}'
```

1. Escolha um nome com 3 a 50 caracteres: minúsculas, dígitos e hífen, sem hífen no começo nem no fim.
2. Guarde o `data.id`: é o `{app}` das outras rotas.
3. Confira se você é admin: `GET /api/v1/applications/{app}/members/me` devolve `"role": "admin"`.

Para criar várias de uma vez, mande uma lista. Itens inválidos voltam em `errors` e o status é `207` ([Lote](../api/batch.md)).

Se o nome já está em uso por uma aplicação ativa, a resposta é `409`.

Referência: [Aplicações](../api/applications.md).
