# Membros

`/api/v1/applications/{app}/members`. Credencial: JWT. Cada rota opera num recurso só (sem lote).

## Listar

`GET /api/v1/applications/{app}/members?page=1&limit=50`. Exige `member.list` (admin ou platform admin; developer recebe `403`).

```json
{
  "data": [
    {"id": "…", "application_id": "…", "user_id": "…", "uri_id": "3f9c0a7e1b2d4c5e8a6b7c8d",
     "role": "admin", "status": "active", "created_at": "2026-11-17T12:00:00Z"}
  ],
  "page": {"page": 1, "limit": 50, "total": 1, "has_next": false, "has_prev": false}
}
```

`invited_by` e `updated_at` só aparecem quando existem.

## Ver o próprio membro

`GET /api/v1/applications/{app}/members/me`. Qualquer membro ativo. É daqui que sai o `uri_id` que o SDK usa. Platform admin que não é membro recebe `404`.

## Atualizar

```http
PATCH /api/v1/applications/{app}/members/{member}
Content-Type: application/json

{"role": "developer", "status": "active"}
```

- Exige `member.update`. `{member}` precisa ser um uuid (`me` não vale aqui).
- JSON estrito: campo desconhecido e mais de um item dão `400`. Pelo menos um de `role` e `status` é obrigatório.
- `role`: `admin` ou `developer`. `status`: `active` ou `suspended`.
- Audita `member.updated` (`role`, `status`).

## Remover

`DELETE /api/v1/applications/{app}/members/{member}`. Exige `member.remove`. Responde `204`.

É um soft delete: apaga o membro, revoga os tokens dele e apaga os segredos. As versões cifradas e a auditoria ficam. Audita `member.removed`.

## Último admin

Uma aplicação sempre tem pelo menos um admin ativo. Rebaixar, suspender ou remover o último dá `409`:

```json
{"error": {"type": "…/errors/#conflict", "title": "Last admin", "status": 409,
           "detail": "An application needs at least one active admin."}}
```

A checagem e a escrita rodam na mesma transação.

## Efeito imediato

O Vault carrega o vínculo do chamador a **cada requisição**. Suspender ou remover um membro vale na hora, mesmo com o JWT dele ainda dentro do prazo, e o token do SDK dele deixa de funcionar.

Arquivos de teste: [`hacks/http/members/`](../../hacks/http/members).
