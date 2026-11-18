# Arquivos `.http`

Cada rota tem um arquivo em [`hacks/http/<service>/<acao>.http`](../../hacks/http), para o cliente HTTP do VS Code/JetBrains. Todos usam `{{APPLICATION_BACKEND}}/api/v1/...` e `Authorization: Bearer {{ACCESS_TOKEN}}` (exceto `runtime/env.http`, que usa `{{SDK_TOKEN}}`).

| Pasta | Arquivos |
|---|---|
| `applications/` | `create`, `list`, `get_by_id`, `update` (PUT), `upgrade` (PATCH), `delete` |
| `members/` | `list`, `me`, `update`, `remove` |
| `invitations/` | `create`, `list`, `revoke`, `accept` |
| `secrets/` | `create`, `create_reference`, `list`, `get`, `rename`, `update`, `reveal`, `delete`, `versions` |
| `tokens/` | `create`, `list`, `revoke` |
| `audit-logs/` | `list` |
| `runtime/` | `env` |

## Variáveis

O `hacks/http/.env` não é versionado: copie o modelo com `cp hacks/http/.env.example hacks/http/.env`. Ele define só `APPLICATION_BACKEND` (`http://localhost:8083`). As outras variáveis o seu cliente HTTP precisa fornecer:

| Variável | O que é |
|---|---|
| `ACCESS_TOKEN` | JWT da conta Murkio |
| `SDK_TOKEN` | token `sk_vt_...` (só `runtime/env.http`) |
| `APP_ID` | id da aplicação |
| `MEMBER_ID` | id do membro (ou use `me`) |
| `SECRET_ID`, `TOKEN_ID`, `INVITATION_ID` | ids dos recursos |
| `URI_ID` | `uri_id` do membro (`members/me`) |

Os arquivos de `applications/` trazem os ids fixos no próprio arquivo.

`applications/create.http` manda um lote de 4 itens de propósito, com um nome inválido (`"1"`), para você ver o `207`.

## Regra

Rota nova = `.http` novo, nomeado pela ação, usando `{{APPLICATION_BACKEND}}`.
