# Formatos de resposta

O formato vem do header `Accept`.

| Formato | `Accept` |
|---|---|
| JSON (padrão) | `application/json` |
| XML | `application/xml`, `text/xml` |
| YAML | `application/x-yaml`, `application/yaml`, `text/yaml` |
| Tabela de texto | `text/plain` |

```sh
curl -H "Authorization: Bearer $ACCESS_TOKEN" -H "Accept: application/yaml" \
  $VAULT/api/v1/applications
```

A negociação respeita os `q`: o maior vence e, em empate, o primeiro da lista. Header vazio, `*/*`, tipo desconhecido ou `q<=0` caem em JSON.

## Rotas só-JSON (segredo, papel ou credencial)

Rota que recebe ou devolve valor de segredo, token, convite ou papel **ignora o `Accept`** e responde sempre JSON. Vale para:

- `POST …/secrets/{id}/reveal`;
- `GET /runtime/{uri_id}/env`;
- `POST …/tokens` (criação);
- `POST …/invitations` (criação) e `POST /invitations/accept`.

Nessas rotas o **corpo da requisição** também é só JSON, o `Content-Type` não decide, e **campo desconhecido é `400`**. Quem manda `role` ou `created_by` onde não cabe não vê o campo ignorado em silêncio.

Nessas rotas, os valores recebidos nunca são repetidos em `invalid_params`, `detail` ou log.

## Corpo das demais rotas

Em rotas comuns (aplicações, por exemplo) o corpo pode ser JSON, XML ou YAML. O `Content-Type` decide e, sem ele, o primeiro byte: `[` ou `{` é JSON, `<` é XML e o resto é YAML. Campo desconhecido é ignorado.

## Formato do envelope

| Situação | Corpo |
|---|---|
| um item | `{"data": {...}}` |
| dois ou mais itens num lote | `{"data": [...]}` |
| listagem | sempre `{"data": [...], "page": {...}}`, nunca desembrulhada |
| um erro | `{"error": {...}}` |
| dois ou mais erros | `{"errors": [...]}` |
| sucesso e erro no mesmo lote | `207` com `data` e `errors` |
| `204` | sem corpo |

Se a renderização falhar, a resposta é um `500` em JSON.
