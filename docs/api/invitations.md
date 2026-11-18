# Convites

Um admin convida alguém por e-mail e entrega um link de uso único. Quem aceita entra com a conta Murkio e vira membro.

## Criar

```http
POST /api/v1/applications/{app}/invitations
Authorization: Bearer <jwt>
Content-Type: application/json

{"email": "joao@example.com", "role": "developer"}
```

```json
{"data": {"id": "…", "application_id": "…", "email": "joao@example.com", "role": "developer",
          "expires_at": "2026-11-24T12:00:00Z", "created_by": "…", "created_at": "2026-11-17T12:00:00Z",
          "token": "inv_Zk3..."}}
```

- Exige `invitation.manage`. Rota **só-JSON**, com corpo estrito (campo desconhecido é `400`).
- `email`: normalizado (trim e minúsculas), até 254 bytes, válido, com um ponto no domínio. `role`: `admin` ou `developer`.
- O `token` (`inv_` + 43 caracteres) **só aparece nesta resposta**. No banco fica o SHA-256 (`token_hash`). Vale 7 dias.
- Na mesma transação grava o convite, o evento de e-mail na outbox (veja [Retenção](../operations/retention.md)) e a auditoria `invitation.created` (`role`).
- Até o Messaging enviar o e-mail, o admin entrega o token ele mesmo.

## Listar

`GET /api/v1/applications/{app}/invitations`. Exige `invitation.manage`. Devolve os 200 mais novos, do mais novo para o mais antigo, **sem paginação**. Cada item traz `status` calculado: `pending`, `accepted`, `revoked` ou `expired`.

## Revogar

`DELETE /api/v1/applications/{app}/invitations/{id}`. Exige `invitation.manage` e responde `204`. Convite inexistente, já aceito ou já revogado dá `404`. Audita `invitation.revoked`.

## Aceitar

```http
POST /api/v1/invitations/accept
Authorization: Bearer <jwt de quem aceita>
Content-Type: application/json

{"token": "inv_Zk3..."}
```

- Qualquer pessoa autenticada com JWT (tokens do SDK são recusados). Responde `201` com o novo membro.
- Token com prefixo errado ou maior que 128 bytes é tratado como inexistente sem tocar o banco.
- Token inexistente, errado, expirado, revogado ou já usado dão o **mesmo** `404`. A tentativa falha é auditada como `access.denied` (`action: invitation.accept`).
- Quem já é membro da aplicação recebe `409` (`Already a member`).
- Em uma transação: marca o convite como aceito (só se ainda estiver pendente e dentro do prazo) e cria o membro com o papel do convite. Audita `invitation.accepted` (`member_id`, `role`).
- Se duas aceitações disputam o mesmo convite, a que perde recebe `404`.
- O e-mail do convite **não** é comparado com a conta de quem aceita: o token é a prova. Veja [limitações](../reference/limitations.md).

Arquivos de teste: [`hacks/http/invitations/`](../../hacks/http/invitations).
