# gRPC interno

Canal privado do Vault para os outros serviços da plataforma (o Identity grava e lê segredos por aqui). **Não é uma API pública.**

- Escuta em `127.0.0.1:GRPC_PORT` (padrão `50052`). O host é fixo e o Caddy não expõe a porta.
- Mensagens de até 64 KiB.
- Pacote `murkio.vault.v1`. O contrato é [`proto/vault/v1/vault.proto`](../../proto/vault/v1/vault.proto) e o código gerado fica em `pkg/rpc/vaultv1/`.
- Mesmas regras das rotas HTTP: permissões, auditoria, lote, limites.

## Credencial

No metadata `authorization`, como `Bearer <credencial>` (o prefixo `bearer` não diferencia maiúsculas):

| Serviço | Credencial aceita |
|---|---|
| `VaultService` | **só** o token do SDK (`sk_vt_...`) |
| todos os outros | **só** o JWT da conta Murkio |

Credencial ausente, inválida ou do tipo errado dá `Unauthenticated`, sempre igual. Um serviço novo nasce JWT-only.

```sh
grpcurl -plaintext -H "authorization: Bearer sk_vt_..." \
  -d '{"uri_id": "3f9c0a7e1b2d4c5e8a6b7c8d"}' \
  127.0.0.1:50052 murkio.vault.v1.VaultService/ReadEnv
```

## Serviços

| Serviço | RPCs | Espelha |
|---|---|---|
| `VaultService` | `ReadEnv` | [Runtime](runtime.md) |
| `ApplicationService` | `CreateApplications`, `GetApplication`, `ListApplications`, `UpdateApplications`, `DeleteApplications` | [Aplicações](applications.md) (não há `Upgrade`/PATCH) |
| `MemberService` | `ListMembers`, `GetMe`, `UpdateMember`, `RemoveMember` | [Membros](members.md) |
| `InvitationService` | `CreateInvitation`, `ListInvitations`, `RevokeInvitation`, `AcceptInvitation` | [Convites](invitations.md) |
| `TokenService` | `CreateToken`, `ListTokens`, `RevokeToken` | [Tokens](tokens.md) |
| `SecretService` | `CreateSecrets`, `ListSecrets`, `GetSecret`, `UpdateSecret`, `RenameSecret`, `DeleteSecret`, `ListSecretVersions`, `RevealSecret` | [Segredos](secrets.md) |
| `AuditService` | `ListAuditLogs` | [Auditoria](audit-logs.md) |

`member_id` aceita `me`.

## Lote e erros

- Em rotas de lote (`CreateApplications`, `CreateSecrets`, ...), cada item é independente. Se todos falham, a chamada devolve o status do primeiro erro. Se ao menos um dá certo, a chamada é `OK` e a resposta traz os sucessos e, em `errors` (`ItemError`), um erro por item que falhou.
- `400` e `422` viram `InvalidArgument`, `401` `Unauthenticated`, `403` `PermissionDenied`, `404` `NotFound`, `409` `AlreadyExists`, `429` `ResourceExhausted`. O resto é `Internal` com o texto fixo `internal error`.
- `x-request-id` (até 128 bytes) vai para a auditoria. Sem ele, o Vault gera um.
- Paginação: `0` é o padrão e valor negativo é `InvalidArgument`.

## Regenerar o código

Veja [Proto](../development/proto.md).

Guia: [Chamar o Vault por gRPC](../guides/call-over-grpc.md).
