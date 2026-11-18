---
name: go-developer
description: Desenvolve e revisa o backend Go do Murkio Vault (API REST em binário único para FreeBSD, SQLite, arquitetura controllers/useCases/repositories com contrato IUseCase e um contrato de repository por service em restful/<service>/contracts/repository.go, sem generics). Use ao criar ou alterar services, use cases, repositories, entities, migrations, rotas em lote, testes e mocks, ou ao revisar código Go deste projeto.
---

# Go developer skill

Guia prático para trabalhar no backend Go do Murkio Vault. **As regras inegociáveis ficam em `.agents/rules/`; esta skill diz como aplicá-las.** Leia as regras relevantes antes de escrever ou revisar código.

## Regras (`.agents/rules/`)
| Arquivo | Leia quando |
|---|---|
| `architecture.md` | criar pastas ou arquivos, mexer em contratos `IUseCase` e de repository (uma interface por service em `restful/<service>/contracts/repository.go`, sem generics), decidir onde algo mora |
| `api.md` | criar ou alterar rotas, lote vs. entidade única, formato de resposta e erros |
| `testing.md` | escrever ou revisar testes e mocks |
| `comments.md` | comentar código ou usar marcadores de seção |
| `production.md` | tocar em `.env`, config, logs, deploy, FreeBSD ou borda (Caddy) |
| `dependencies.md` | adicionar ou trocar dependências |
| `commits.md` | fazer commit, PR ou qualquer coisa que registre autoria (a IA nunca é co-autora) |

## Fluxo: criar um novo service REST (`<service>`)
1. Entity em `internal/entities/<service>.go` (validação no próprio arquivo) e erros em `errors.go` seguindo RFC 9457.
2. Migration em `migrations/sqlite/NNNN_<service>_table.up.sql` e o `.down.sql` correspondente.
3. Contrato em `internal/restful/<service>/contracts/repository.go`, uma única interface `I<Recurso>Repository` com todos os métodos do service (sem generics); depois o repository em `repositories/repository.go`, implementando-a.
4. Um arquivo por caso de uso em `useCases/`, aceitando lote ou entidade única (`api.md`).
5. `services/` com o parse de path, body e paginação, devolvendo `entities.Error`.
6. Controller HTTP em `http_controllers/controller.go` (e o gRPC, quando houver, em `grpc_controllers/controller.go`) registrando as rotas `/api/v1/<service>` (com e sem `{id}`, com e sem barra final).
7. `mocks/mock.go` (um único arquivo) com o fake do repository, dos use cases e os helpers.
8. Testes de cada use case e do controller (`testing.md`).
9. `initializr.go` do service e registro no `cmd/api/main.go`.
10. `.http` em `hacks/http/<service>/<acao>.http`.
11. Documentação: `docs/api/<service>.md` e as linhas do service em `docs/api.md` (e a matriz de permissões, se houver ação nova em `entities.Decide`).
12. Antes de terminar: `gofmt -l .`, `go vet ./...` e `go test -race ./...`.

## Checklist de revisão
- Repositories sem generics: cada use case injeta a interface do service (`restful/<service>/contracts/repository.go`) num campo `Repository`, nunca o tipo concreto.
- Camadas respeitadas: `http_controllers`/`grpc_controllers -> useCases -> contracts/entities`; só o `initializr.go` conhece implementações concretas.
- `Perform` sem `error`, com `Data` e `Errors`/`Error` no Output.
- Itens do lote independentes; id duplicado e uuid zero tratados.
- `sql.ErrNoRows` vira 404 e outro erro vira 500, sem vazar a mensagem interna.
- Testes cobrem sucesso, sucesso parcial, falha, dados válidos e inválidos aleatórios.
- Comentários: godoc de 5 linhas ou mais só onde a função foge do padrão; comentário curto só para o motivo não óbvio de uma linha ou consulta; marcadores só nos arquivos predefinidos (`comments.md`).
- Rota, limite, variável ou regra alterados têm a página de `docs/` atualizada.
- Nenhum commit ou PR cita a IA como autora ou co-autora (`commits.md`).
- Nada de lógica de borda (rate limit, log de acesso, TLS) dentro do vault.

## Comandos
O `makefile` do vault só inclui `hacks/make/proto.mk`. O resto roda direto com `go`:
```bash
go run ./cmd/api                      # sobe a API (HTTP e gRPC)
go build -o bin/vault-api ./cmd/api   # binário (GOOS=freebsd GOARCH=amd64 para produção)
gofmt -l . && go vet ./... && go test -race ./...
make proto                            # regenera pkg/rpc/vaultv1 a partir de proto/ (precisa de protoc)
```

Planejados, ainda não existem: `make api|cli|mcp|jobs`, `make build`, `make sudo-delete-database|sudo-delete-tables|sudo-delete-<tabela>`, `make gen-data` (e os entrypoints `cmd/cli`, `cmd/mcp`, `cmd/jobs`). Quando existirem, atualize este bloco.

## Documentação
A documentação do serviço está em `README.md` e `docs/` (índice em `docs/README.md`, contrato HTTP em `docs/api.md`). Mudou uma rota, um limite, uma variável ou uma regra? Atualize a página correspondente na mesma mudança.
