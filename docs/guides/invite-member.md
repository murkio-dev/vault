# Convidar um membro

Só admin (ou platform admin) convida.

```sh
curl -s -X POST $VAULT/api/v1/applications/$APP_ID/invitations \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email": "joao@example.com", "role": "developer"}'
```

1. Escolha o papel: `developer` (só a própria tabela) ou `admin` (tudo na aplicação).
2. Copie o `data.token` (`inv_...`). Ele **aparece só nesta resposta** e vale 7 dias.
3. Entregue o token a quem vai aceitar. Enquanto o Messaging não envia o e-mail, a entrega é sua.
4. Acompanhe em `GET …/invitations`: o `status` vai de `pending` para `accepted` (ou `expired`/`revoked`).

Para cancelar: `DELETE …/invitations/{id}`.

Quem recebe segue [Aceitar um convite](accept-invitation.md). Referência: [Convites](../api/invitations.md).
