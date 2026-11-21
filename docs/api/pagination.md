# Paginação

Listagens sempre devolvem um array em `data`, nunca um objeto solto.

## `page` e `limit` (padrão)

```sh
curl "$VAULT/api/v1/applications?page=2&limit=50" -H "Authorization: Bearer $ACCESS_TOKEN"
```

```json
{
  "data": [ ... ],
  "page": {"page": 2, "limit": 50, "total": 120, "has_next": true, "has_prev": true}
}
```

- `page` começa em `1` e `limit` padrão é `150` (também o máximo: valores maiores são cortados em 150).
- `page` ou `limit` que não seja inteiro, ou seja menor que 1, dá `400` apontando o campo.

Usam esse formato: lista de aplicações e de membros.

## Cursor (auditoria)

`GET …/audit-logs` pagina por cursor, em ordem do mais novo para o mais antigo:

```sh
curl "$VAULT/api/v1/applications/$APP_ID/audit-logs?limit=100&cursor=<next_cursor>" -H "Authorization: Bearer $ACCESS_TOKEN"
```

```json
{"data": [ ... ], "page": {"next_cursor": "0194f3a2-..."}}
```

- `limit` padrão `100`, máximo `200`. Um valor acima de 200 volta ao padrão de 100 (veja [limitações](../reference/limitations.md)).
- `next_cursor` é o id da última linha devolvida, ou `null` quando acabou.
- Passe `cursor=<next_cursor>` para a página seguinte.

## Listas sem paginação

| Rota | O que devolve |
|---|---|
| `GET …/invitations` | os 200 convites mais novos |
| `GET …/secrets` | todos os segredos do membro (até 100) |
| `GET …/secrets/{id}/versions` | até 500 versões, da mais nova à mais antiga |
| `GET /api-keys` | até 200 chaves, inclusive as apagadas |
| `GET /runtime/{uri_id}/env` | até 100 variáveis, ordenadas por nome |

No gRPC, `page` e `limit` valem `0` para "padrão" e negativo é `InvalidArgument`.
