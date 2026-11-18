# Permissões

Toda decisão de permissão passa por `entities.Decide` (`internal/entities/policy.go`): uma função pura, testada em matriz. A pessoa chama o use case `Authorize` (service `member`), injetado nos demais services como `contracts.IAccessGuard`. Nenhum use case compara papel por conta própria.

## Quem decide o quê

1. O `Authorize` carrega o vínculo do chamador **a cada chamada**. Remover ou suspender um membro vale na hora.
2. O papel dentro da aplicação vem do banco do Vault (`members.role`). O JWT só entra com o role `admin` do Identity, que faz da pessoa **platform admin**.
3. `Decide` devolve uma de três respostas: permitido, **proibido** (`403`) ou **oculto** (`404`).

| Chamador | Resultado |
|---|---|
| platform admin | pode tudo |
| admin da aplicação | pode tudo nela, inclusive nos segredos e tokens dos outros membros |
| developer ativo | lê a aplicação e age só nos próprios segredos, tokens e linhas de auditoria. Gerir membros, convites e a aplicação dá `403` |
| não membro, suspenso ou removido | `404` (oculto, igual a um recurso inexistente) |
| token do SDK | só `env.read`, e só do próprio membro ativo |
| ação desconhecida | negada para todos |

`application.create` e `invitation.accept` valem para qualquer pessoa autenticada, mas **nunca** para token do SDK.

## Dono do dado

Rotas com `{member}` (`secrets`, `tokens`) passam o dono como parâmetro explícito. `me` vira o próprio chamador. Se o dono não existir na aplicação, a resposta é `404`.

## Acesso negado é auditado

Um acesso negado grava `access.denied` (`outcome: denied`, com `action` e `status` no `metadata`). Se gravar essa linha falhar, a resposta de erro não muda.

## Último admin

Rebaixar, suspender ou remover o último admin ativo dá `409` (`Last admin`). Veja [Membros](../api/members.md).

## Matriz completa

A tabela por ação está em [API HTTP](../api.md#matriz-de-permissões).
