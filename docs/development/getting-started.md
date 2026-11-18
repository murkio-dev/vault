# Primeiros passos (desenvolvimento)

## Pré-requisitos

- Go 1.27.
- Para mexer no gRPC: `protoc`, `protoc-gen-go` e `protoc-gen-go-grpc` no `PATH`.
- Um Identity rodando (porta `8080`) com a aplicação do Vault cadastrada, para ter o `AUTH_APPLICATION_ID` e JWTs reais. Os testes não precisam dele: eles sobem um Auth falso (`pkg/authn/authntest`).

## Rodar

```sh
cp .env.example .env     # preencha ENCRYPTION_KEY e AUTH_APPLICATION_ID
go run ./cmd/api
```

Da raiz do monorepo, `make go-up` sobe o identity, o messaging e o vault juntos com `go run`.

## Comandos

| Comando | O que faz |
|---|---|
| `go run ./cmd/api` | sobe a API (HTTP e gRPC) |
| `go build -o bin/vault-api ./cmd/api` | compila o binário |
| `gofmt -l .` | lista arquivos fora do formato (deve sair vazio) |
| `go vet ./...` | análise estática |
| `go test -race ./...` | todos os testes, com detector de corrida |
| `make proto` | regenera o código gRPC ([Proto](proto.md)) |

O `makefile` do vault só inclui `hacks/make/proto.mk`. Os alvos `api`, `build`, `sudo-delete-*` e `gen-data` ainda não existem. Veja [Limitações](../reference/limitations.md).

Antes de terminar qualquer mudança em Go: `gofmt -l .`, `go vet ./...` e `go test -race ./...`.

## Testar na mão

Os arquivos de [`hacks/http/`](../../hacks/http) cobrem todas as rotas ([Arquivos `.http`](http-files.md)).

## Onde mexer

| Quero… | Vá para |
|---|---|
| mudar uma regra de um recurso | `internal/restful/<service>/useCases/` |
| mudar o acesso ao banco | `internal/restful/<service>/repositories/repository.go` e `contracts/repository.go` |
| mudar quem pode o quê | `internal/entities/policy.go` |
| mudar um erro | `internal/entities/errors.go` |
| criar uma tabela | `migrations/sqlite/NNNN_<nome>.up.sql` e `.down.sql` |
| ajustar o servidor, formatos ou paginação | `pkg/httpx/` |
| criar um service novo | [Criar um novo service](../guides/add-a-new-service.md) |

## Regras do projeto

Leia antes de codar: [`.agents/skills/go-developer.md`](../../.agents/skills/go-developer.md) e [`.agents/rules/`](../../.agents/rules). Resumo em [Arquitetura](architecture.md) e [Testes](testing.md).
