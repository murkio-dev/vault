# Pacotes (`pkg/`)

Infra e helpers que **não conhecem o domínio** do Vault. `pkg/` pode usar `internal/entities`, mas nunca importa `internal/restful` nem `internal/adapters`.

| Pacote | Para que serve |
|---|---|
| [`httpx`](#httpx) | entrega HTTP: respostas, formatos, binders, rotas, servidor |
| [`grpcx`](#grpcx) | entrega gRPC: servidor interno, interceptor, helpers |
| [`authn`](#authn) | JWT do Identity, JWKS em cache e o middleware |
| [`secretbox`](#secretbox) | AES-256-GCM com chaveiro |
| [`sqlite`](#sqlite) | abertura do banco, DSN, transações |
| [`migration`](#migration) | executor das migrations embutidas |
| [`cappedfile`](#cappedfile) | arquivo de log com teto de linhas |
| [`graceful`](#graceful) | sobe vários servidores e os desliga juntos |
| [`dependencies`](#dependencies) | monta o `Dependencies` (guard, transação auditada, cifra) e o autenticador do SDK |
| [`mocks`](#mocks) | fake de `IUseCase`, `NewGRPCConn` e geradores aleatórios |
| `rpc/vaultv1` | código gerado do `.proto` (versionado, nunca editado à mão) |

## httpx

- `respond.go`: `Respond` (status do sucesso, desembrulha item único), `RespondList` (nunca desembrulha), `RespondJSON` (sempre JSON), `RespondError`.
- `formats.go`: negociação por `Accept` (JSON, XML, YAML, tabela).
- `bindJson.go`: `BindBody[T]` (formato pelo `Content-Type` ou pelo primeiro byte), `BindJSON[T]` (só JSON, campo desconhecido é erro) e `BindIDs`.
- `request.go`: `PathUUID`, `PathMember` (aceita `me`), `BindOne`, `BindOptional`, `Pagination`, `PageWindow`.
- `routes.go`: `Handle` registra o padrão com e sem `/{$}`.
- `params.go`: `AssertNotNilParams`, usado também pelos `grpc_controllers`.
- `recovery.go`: pânico vira `500` (RFC 9457), sem expor o texto. `http.ErrAbortHandler` é repropagado.
- `server.go`: `NewServer` com timeouts, teto de corpo (64 KiB) e de cabeçalho (1 MiB), recovery e log de boot. Implementa `graceful.Server`.

## grpcx

- `server.go`: servidor interno (`127.0.0.1`, mensagens de até 64 KiB), `Shutdown` com `GracefulStop` e fallback para `Stop`.
- `authenticate.go`: o interceptor. `VaultService` aceita só o token do SDK, os demais só o JWT.
- `errors.go`: `Error` (status HTTP para código gRPC) e `ItemErrors`.
- `requester.go`: monta `entities.ApiHeader` a partir do metadata.
- `parse.go`: `UUID`, `MemberID` (aceita `me`), `Pagination`.
- `messages.go`: conversores para as mensagens (`Page`, `Time`, `MemberMessage`...).

## authn

- `authn.go`: `Verifier` (EdDSA, `iss`, `aud`, `aid`, expiração) e o cache do JWKS.
- `middleware.go`: `Authenticate` (só em `/api/`, com `no-store`) e `Identify`, a regra única das duas entregas: o prefixo `sk_vt_` decide o tipo.
- `authntest/`: um Auth falso para os testes.

## secretbox

`FromBase64(chaveAtiva, anteriores)`, `Seal`, `Open` e `OpenStale` (que avisa que abriu com chave antiga). Erros `ErrInvalidKey` e `ErrInvalidSealed`. Veja [Cifragem](../security/encryption.md).

## sqlite

`New` (WAL, conexão única), `DSN`, `IsUniqueViolation`, `IsConstraintViolation`, e `Transactor`/`Conn` (a transação viaja no `ctx`). `sqlitetest/` entrega um banco **migrado** para os testes e o `Seed` com dois tenants.

## migration

Lê `sqlite/*.up.sql` do FS embutido, em ordem de nome, uma transação por arquivo, e registra em `schema_migrations`.

## cappedfile

`Append(path, linha, maxLinhas)` com `flock`: acrescenta e mantém só as últimas N linhas.

## graceful

`Run(ctx, timeout, servers...)` sobe todos e, com `SIGTERM`, `Ctrl+C`, contexto cancelado ou um servidor que sai, chama `Shutdown` em todos em paralelo.

## dependencies

`dependencies.New(db, box)` devolve o `contracts.Dependencies` que o `restful.Init` injeta nos services e o autenticador do token do SDK. É chamada pelo `cmd/api/main.go` (HTTP e gRPC) e pelos testes. Ela conhece repositories e adapters de propósito: é a única peça de `pkg/` que faz isso.

## mocks

`pkg/mocks/mock.go`: o fake genérico de `IUseCase` (controllers), `TokenAuthenticator`, `NewGRPCConn`, `NewDependencies(t, db)` (Dependencies de verdade para os `initializr_test`) e os geradores `GenRandom...` usados nos testes e nos `mocks/` dos services. Dado novo em teste = gerador novo aqui.
