# Aceitar um convite

Você precisa de uma conta Murkio e do token `inv_...` que o admin enviou.

```sh
curl -s -X POST $VAULT/api/v1/invitations/accept \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"token": "inv_..."}'
```

A resposta (`201`) é o seu novo membro, com `uri_id`, `role` e `status`.

| Resposta | Motivo |
|---|---|
| `404` | token errado, expirado, revogado ou já usado (todos iguais, de propósito) |
| `409` | você já é membro dessa aplicação |
| `401` | JWT inválido (o token do SDK não serve aqui) |

O convite é de uso único. Se alguém aceitar o mesmo convite ao mesmo tempo, só um consegue.

Referência: [Convites](../api/invitations.md).
