# Conceitos

## Aplicação

A unidade de isolamento (tenant). O nome tem de 3 a 50 caracteres, em minúsculas, dígitos e hífen, sem hífen no começo nem no fim (`^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$`) e é **único entre todas as aplicações ativas**. Apagar uma aplicação é um soft delete: o nome volta a ficar livre.

Rotas: [Aplicações](../api/applications.md).

## Membro e papel

Quem participa de uma aplicação. Cada membro tem:

- `role`: `admin` ou `developer`;
- `status`: `active` ou `suspended`;
- `uri_id`: 24 caracteres hexadecimais, o identificador público da tabela de segredos dele.

Uma aplicação sempre tem pelo menos um admin ativo: rebaixar, suspender ou remover o último admin dá `409`.

Rotas: [Membros](../api/members.md). Permissões: [Permissões](../security/permissions.md).

## Convite

Um link de uso único, com token `inv_...`, válido por 7 dias. O token só aparece na resposta da criação. O status (`pending`, `accepted`, `revoked`, `expired`) é calculado, não gravado.

Rotas: [Convites](../api/invitations.md).

## Segredo

Um par nome/valor na tabela de um membro. O nome segue `^[A-Z][A-Z0-9_]{0,127}$` (`DATABASE_URL`). O tipo é um de `string`, `password`, `url`, `database_uri`, `api_key`, `jwt_key`, `paseto_key`.

Cada segredo tem uma **origem** (`source`):

| Origem | Significado | Disponível |
|---|---|---|
| `static` | O valor foi informado por uma pessoa (padrão) | sim |
| `reference` | Aponta para outro segredo `static` da mesma aplicação e do mesmo tipo | sim |
| `generated` | O Vault gera o valor a partir do tipo | não (`400`) |
| `dynamic` | O Vault cria credenciais no banco do cliente | não (`400`) |

Cada mudança de valor cria uma **nova versão**. As anteriores ficam cifradas e imutáveis. Listas **nunca** trazem o valor: só a ação de revelar e o endpoint de runtime devolvem.

Rotas: [Segredos](../api/secrets.md).

## URI e token

- A **URI** (`uri_id`) é um identificador público e estável da tabela de um membro. Ela não carrega senha.
- A **chave secreta** (`sk_vt_` + 43 caracteres) é a credencial. A `pk_vt_` só identifica a aplicação e nunca lê segredo. Só o SHA-256 dele fica no banco, ele aparece uma única vez e pode ter validade (até 10 anos).

O SDK chama `GET /api/v1/runtime/{uri_id}/env` com `Authorization: Bearer sk_vt_...`. Um token só lê a `uri_id` do próprio membro.

Rotas: [Chaves de API](../api/api-keys.md) e [Runtime](../api/runtime.md).

## Auditoria

Toda escrita grava, na mesma transação, uma linha em `audit_logs`. Se a linha não puder ser gravada, a operação não acontece (fail-closed). A tabela é append-only e guarda ids e nomes, nunca valores.

Rotas: [Auditoria](../api/audit-logs.md).
