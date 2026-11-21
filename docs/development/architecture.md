# Arquitetura

Um binário Go, em camadas, com **uma pasta autocontida por service**.

```text
cmd/api/main.go              entrypoint: só liga as peças
internal/
  contracts/                 IUseCase, IAccessGuard, IAuditedTx, ISecretValues, Dependencies
  entities/                  domínio, erros (RFC 9457), paginator, policy (Decide), logs
  adapters/                  audited, secretvalues, tokenauth (implementam contratos globais)
  config/                    variáveis de ambiente e caminhos (só o main usa)
  restful/                   entrega HTTP e gRPC
    initializr.go            monta Dependencies, Init (HTTP) e InitGRPC
    <service>/               application, member, invitation, secret, token, runtime, audit
      contracts/repository.go
      http_controllers/      rotas HTTP
      grpc_controllers/      handlers gRPC
      services/              parse e validação de request
      useCases/              um arquivo por caso de uso
      repositories/          SQLite
      mocks/mock.go          fakes do service
      initializr.go
migrations/sqlite/           *.up.sql e *.down.sql, embutidos no binário
pkg/                         infra sem domínio: httpx, grpcx, authn, secretbox, sqlite...
proto/vault/v1/vault.proto   contrato do gRPC (o vault é o dono)
hacks/http/                  requisições .http de cada rota
```

O `runtime` não tem `repositories/` (ele usa os dos outros services). Os demais seguem o molde completo.

## Dependências entre camadas

```text
http_controllers / grpc_controllers  ->  useCases  ->  contracts / entities
repositories                         ->  contracts / entities
internal/adapters                    ->  contracts / entities / pkg
```

- Só o `initializr.go` conhece as implementações concretas.
- `internal/entities` pode ser importada por qualquer pacote.
- `restful/<service>/contracts` depende só de `entities` e pode ser importado por qualquer um.
- `pkg/` nunca importa `internal/restful` nem `internal/adapters`.
- Não existe `internal/middlewares/`: autenticação vive em `pkg/authn` e recovery em `pkg/httpx`. Rate limit, log de acesso e TLS são do Caddy.

## `IUseCase`

O contrato de toda regra de negócio (o único lugar com generics, junto com os helpers de `pkg/`):

```go
type IUseCase[i any, o any] interface {
    Perform(ctx context.Context, input i) o
}
```

`Perform` **não retorna `error`**. Como as operações são em lote, o `Output` carrega `Data` e `Errors` (ou `Error` para item único). O controller entrega o output para `httpx.Respond`/`RespondList`, que decide o status (200, 201, 204, 207 ou o do primeiro erro). DTOs: `<Nome>Input` (sempre com `Requester entities.ApiHeader`) e `<Nome>Output`.

## Repositories

- Um contrato por service, em `restful/<service>/contracts/repository.go`: **uma única interface** `I<Recurso>Repository`, sem generics. O pacote se chama `<service>contracts`.
- Vocabulário: `Create`, `Exist`, `FindById`, `List`, `Update` (substituição, PUT), `Upgrade` (parcial, PATCH) e `Delete`. Operação que não é CRUD tem o nome da intenção (`Revoke`, `Accept`...). Atomicidade entre passos fica **dentro** de um método do repository.
- Registro não encontrado devolve `sql.ErrNoRows` (o use case transforma em `404`, e qualquer outro erro em `500`).
- O repository declara `var _ <service>contracts.I<Recurso>Repository = Repository{}`.
- A aplicação (e o dono) são **parâmetros explícitos**. Nada de tenant no `ctx`. Uuid zero é recusado.

## Peças compartilhadas

| Peça | Onde | Papel |
|---|---|---|
| **Autorização** | `member/useCases/authorize.go` | `Authorize` usa `entities.Decide`. É injetada nos outros services como `contracts.IAccessGuard` |
| **Auditoria** | `adapters/audited` | `IAuditedTx.Run` roda a mudança e grava a linha da trilha na mesma transação |
| **Cifragem** | `adapters/secretvalues` | ponte entre o ciphertext e o `pkg/secretbox` |
| **Token do SDK** | `adapters/tokenauth` | o autenticador do `sk_vt_` para o middleware e o gRPC |
| **Dependencies** | `contracts/dependencies.go` | o que o `initializr` raiz injeta nos services (`Guard`, `Tx`, `Values`, `Audit`, `Members`, `Secrets`) |

## Boot (`cmd/api/main.go`)

1. `config.New()`: lê o ambiente e o `.env` e valida.
2. Abre o log (`vault.log`) e o SQLite.
3. Carrega a `ENCRYPTION_KEY` no `secretbox`.
4. Roda as migrations embutidas.
5. `restful.Init(...)`: registra as rotas HTTP de cada service e devolve o autenticador do token.
6. Abre o cliente gRPC do Identity (`IDENTITY_GRPC_ADDR`) e cria o verificador de JWT (`AUTH_APPLICATION_ID`).
7. `restful.InitGRPC(...)`: sobe o gRPC em `127.0.0.1`.
8. `httpx.NewServer`, com o handler `authn.Authenticate(verifier, tokens, mux)`.
9. `graceful.Run`: sobe os dois servidores e os desliga juntos por sinal.

O `dependencies.New` (`pkg/dependencies`) é chamado duas vezes (HTTP e gRPC): os repositories não têm estado.

## Convenções de código

- **Marcadores** de seção nos arquivos predefinidos: `// Builder`, `// DTOS`, `// UseCase` (use cases), `// Builder`, `// Functions` (repositories), `// Main test`, `// Functions` (testes), `// Mocks`, `// Helpers` (mocks).
- **Comentário é exceção**: só o que foge do padrão da arquitetura.
- **Testes sempre em par**: todo `_test.go` tem um `.go` de mesmo nome ao lado.
- **Sem dependência de terceiros** além das aprovadas em [`dependencies.md`](../../.agents/rules/dependencies.md).

Veja [Pacotes](packages.md) e [Testes](testing.md).
