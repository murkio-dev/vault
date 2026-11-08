---
name: go-developer
description: Desenvolve e revisa o backend Go do Murkio Vault (API REST em binário único para FreeBSD, SQLite, arquitetura controllers/useCases/repositories com contrato IUseCase e contratos de repository específicos por grupo, sem generics). Use ao criar ou alterar services, use cases, repositories, entities, migrations, rotas em lote, testes e mocks, ou ao revisar código Go deste projeto.
---

# Go developer skill

Guia prático para trabalhar no backend Go do Murkio Vault. **As regras inegociáveis ficam em `.agents/rules/`; esta skill diz como aplicá-las.** Leia as regras relevantes antes de escrever ou revisar código.

## Regras (`.agents/rules/`)
| Arquivo | Leia quando |
|---|---|
| `architecture.md` | criar pastas ou arquivos, mexer em contratos `IUseCase` e de repository (um por grupo, sem generics), decidir onde algo mora |
| `api.md` | criar ou alterar rotas, lote vs. entidade única, formato de resposta e erros |
| `testing.md` | escrever ou revisar testes e mocks |
| `comments.md` | comentar código ou usar marcadores de seção |
| `production.md` | tocar em `.env`, config, logs, deploy, FreeBSD ou borda (Caddy) |
| `dependencies.md` | adicionar ou trocar dependências |
| `commits.md` | fazer commit, PR ou qualquer coisa que registre autoria (a IA nunca é co-autora) |

## Fluxo: criar um novo service REST (`<service>`)
1. Entity em `internal/entities/<service>.go` (validação no próprio arquivo) e erros em `errors.go` seguindo RFC 9457.
2. Migration em `migrations/sqlite/NNNN_<service>_table.up.sql` e o `.down.sql` correspondente.
3. Repository em `internal/restful/<service>/repositories/repository.go`, implementando os contratos do recurso declarados em `internal/contracts/repository.go` (interfaces pequenas por grupo, sem generics).
4. Um arquivo por caso de uso em `useCases/`, aceitando lote ou entidade única (`api.md`).
5. `services/` com o parse de path, body e paginação, devolvendo `entities.Error`.
6. Controller em `controllers/controller.go` registrando as rotas `/api/v1/<service>` (com e sem `{id}`, com e sem barra final).
7. `mocks/` com o fake do repository e dos use cases.
8. Testes de cada use case e do controller (`testing.md`).
9. `initializr.go` do service e registro no `cmd/api/main.go`.
10. `.http` em `hacks/http/<service>/<acao>.http`.
11. Antes de terminar: `gofmt -l .`, `go vet ./...` e `go test -race ./...`.

## Checklist de revisão
- Repositories sem generics: cada use case injeta só os contratos específicos que usa (`contracts/repository.go`), nunca o tipo concreto.
- Camadas respeitadas: `controllers -> useCases -> contracts/entities`; só o `initializr.go` conhece implementações concretas.
- `Perform` sem `error`, com `Data` e `Errors`/`Error` no Output.
- Itens do lote independentes; id duplicado e uuid zero tratados.
- `sql.ErrNoRows` vira 404 e outro erro vira 500, sem vazar a mensagem interna.
- Testes cobrem sucesso, sucesso parcial, falha, dados válidos e inválidos aleatórios.
- Comentários só onde foge do padrão, com 5 linhas ou mais; marcadores só nos arquivos predefinidos.
- Nenhum commit ou PR cita a IA como autora ou co-autora (`commits.md`).
- Nada de lógica de borda (rate limit, log de acesso, TLS) dentro do vault.

## Comandos Make
```bash
make api # Inicia o servidor de api
make cli <comand> # executa algum comando do CLI
make mcp # Inicia o servidor mcp
make jobs # inicia os workers

make build (api, cli, mcp, jobs) # Builda para o freeBSD seguindo todas as regras de .agents/rules

make sudo-delete-database # Deleta o banco de dados inteiro
make sudo-delete-tables # Deleta a tabela
make sudo-delete-<nome da tabela q vai deletar os dados dela> # Deleta dados de uma tabela
make gen-data # Preenche o banco de dados com dados validos e aleatorios.

```
