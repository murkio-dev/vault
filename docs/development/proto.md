# Proto (gRPC)

O Vault é o **dono** do contrato gRPC. O arquivo fica em [`proto/vault/v1/vault.proto`](../../proto/vault/v1/vault.proto) (pacote `murkio.vault.v1`) e **só o dono edita**. Quem consome (o Identity) copia o arquivo.

O código gerado (`pkg/rpc/vaultv1/`) é **versionado**: nunca se gera no boot nem no build, e nunca se edita à mão.

## Regenerar

Pré-requisito: `protoc`, `protoc-gen-go` e `protoc-gen-go-grpc` no `PATH`.

```sh
make proto
```

O alvo (`hacks/make/proto.mk`) apaga `pkg/rpc/vaultv1`, roda o `protoc` com `--go_out` e `--go-grpc_out` (`paths=source_relative`) e move os arquivos para `pkg/rpc/vaultv1/`.

## Mudar o contrato

1. Edite `proto/vault/v1/vault.proto` (só acrescente campos novos, sem reaproveitar números).
2. `make proto`.
3. Implemente o handler em `restful/<service>/grpc_controllers/controller.go` e registre em `InitGRPC`.
4. Escreva o teste do controller (com `pkgmocks.NewGRPCConn`).
5. Copie o `.proto` para o Identity (`make proto-sync`/`make proto-check`, lá).
6. Atualize [gRPC interno](../api/grpc.md).

## Credenciais

O interceptor (`pkg/grpcx/authenticate.go`) decide pelo nome do método: `/murkio.vault.v1.VaultService/*` aceita só o token do SDK, todo o resto aceita só o JWT. Um serviço novo nasce JWT-only.

> Os comentários do `.proto` ainda falam em `vt_...` e dizem que o JWT vale "só" no `ApplicationService`. O prefixo real é `sk_vt_` e o JWT vale em todos os serviços exceto `VaultService`.
