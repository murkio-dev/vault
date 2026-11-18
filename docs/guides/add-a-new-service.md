# Criar um novo service

O fluxo que a skill [`go-developer`](../../.agents/skills/go-developer.md) exige para um recurso REST novo (`<service>`). Leia antes as [regras](../../.agents/rules).

1. **Entity** em `internal/entities/<service>.go` (com a validação) e os erros em `errors.go`, no formato RFC 9457.
2. **Migration** `migrations/sqlite/NNNN_<service>_table.up.sql` e o `.down.sql`.
3. **Contrato** em `internal/restful/<service>/contracts/repository.go`: uma única interface `I<Recurso>Repository`, sem generics. Depois o repository em `repositories/repository.go`, com `var _ ... = Repository{}`.
4. **Use cases**: um arquivo por caso de uso em `useCases/`, com os marcadores `// Builder`, `// DTOS`, `// UseCase`, aceitando lote ou entidade única.
5. **`services/`**: parse de path, body e paginação, devolvendo `entities.Error`.
6. **Controller HTTP** em `http_controllers/controller.go` (e o gRPC, se houver, em `grpc_controllers/`), registrando `/api/v1/<service>` com `httpx.Handle` (com e sem `{id}`, com e sem barra final).
7. **`mocks/mock.go`**: um arquivo só, com o fake do repository, dos use cases e os helpers.
8. **Testes** de cada use case e do controller ([Testes](../development/testing.md)).
9. **`initializr.go`** do service e o registro em `restful/initializr.go`.
10. **`.http`** em `hacks/http/<service>/<acao>.http`.
11. **Documentação**: a página em `docs/api/<service>.md` e as linhas em [`docs/api.md`](../api.md).
12. **Antes de terminar**: `gofmt -l .`, `go vet ./...` e `go test -race ./...`.

Checklist de revisão:

- cada use case injeta a interface do service num campo `Repository`, nunca o tipo concreto;
- camadas respeitadas (`controllers -> useCases -> contracts/entities`);
- `Perform` sem `error`, com `Data` e `Errors`/`Error` no `Output`;
- itens do lote independentes, id duplicado e uuid zero tratados;
- `sql.ErrNoRows` vira `404` e outro erro `500`, sem vazar a mensagem;
- nenhuma lógica de borda (rate limit, log de acesso, TLS).

Visão geral: [Arquitetura](../development/architecture.md).
