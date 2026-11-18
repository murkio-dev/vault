# Chamar o Vault por gRPC

O gRPC é para serviços da plataforma, em `127.0.0.1:50052`.

## Ler o ambiente (token do SDK)

```sh
grpcurl -plaintext \
  -H "authorization: Bearer sk_vt_..." \
  -import-path proto -proto vault/v1/vault.proto \
  -d '{"uri_id": "3f9c0a7e1b2d4c5e8a6b7c8d", "names": ["DATABASE_URL"]}' \
  127.0.0.1:50052 murkio.vault.v1.VaultService/ReadEnv
```

## Administrar (JWT da pessoa)

```sh
grpcurl -plaintext \
  -H "authorization: Bearer $ACCESS_TOKEN" \
  -import-path proto -proto vault/v1/vault.proto \
  -d '{"applications": [{"name": "loja-api"}]}' \
  127.0.0.1:50052 murkio.vault.v1.ApplicationService/CreateApplications
```

(Veja os campos exatos de cada mensagem em [`vault.proto`](../../proto/vault/v1/vault.proto).)

## Em Go

Copie o `.proto` e gere o cliente (o Identity faz isso em `identity/pkg/rpc/vaultv1`, com o cliente em `identity/pkg/vaultclient`). Mande a credencial no metadata `authorization`:

```go
ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+jwt)
```

Erros: `Unauthenticated` (credencial errada ou do tipo errado), `PermissionDenied`, `NotFound`, `InvalidArgument`, `AlreadyExists`. Em lote, a resposta `OK` traz os sucessos e um `ItemError` por falha.

Referência: [gRPC interno](../api/grpc.md).
