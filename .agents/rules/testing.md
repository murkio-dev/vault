# Regras: testes

Todo código novo vem com testes, no mesmo diretório do arquivo testado (`<arquivo>_test.go`), em pacote externo (`package useCases_test`) e com `testify/assert` (exceção aprovada em `dependencies.md`, só em testes). Fakes ficam em `mocks/` do service e nunca são duplicados dentro de arquivos `_test.go`. Meta: 100% de cobertura nos use cases. Rodar sempre com `go test -race ./...`.

## Estrutura
- Uma função `Test<Nome>` por unidade, com um `t.Run("Quando ...", ...)` por cenário em português, cada um chamando uma função `when<Cenario>(t)`.
- Dados do request vêm de `mocks.NewRequester()` e `mocks.NewApp()`, que usam geradores aleatórios de `pkg/utils/gen.go` (`GenRandom...`). Novo tipo de dado = novo gerador em `gen.go`.

## Cenários obrigatórios de cada use case
1. **Build**: o construtor injeta os contratos de repository que o use case usa (e aceita nil sem panic na construção).
2. **Funciona (sucesso total)**: entidade única e lote com dados aleatórios **válidos**. Confere `Data`, ordem do lote preservada, `Errors` vazio e chamadas ao repositório.
3. **Funciona parcialmente**: lote misto (válido + inválido + não encontrado/conflito + erro de banco). `Data` só com os sucessos, um erro por item que falhou, na ordem do lote.
4. **Não funciona**: entidade única e lote com dados aleatórios **inválidos** (vazio, caracteres proibidos, uuid zero, id duplicado). `Data` nil, `Errors` com o `Type`/`Status` certo, e o repositório **não** é chamado.
5. **Falhas do repositório**: `sql.ErrNoRows` vira 404 e qualquer outro erro vira 500, sem vazar a mensagem interna no `Detail`.
6. **Entrada vazia**: lista vazia não gera `Data` nem `Errors`.
7. **Lote com id duplicado**: reportado uma vez, nenhuma ocorrência processada, os outros itens seguem.
8. **Listagem**: valores padrão de `page`/`limit`, limite máximo, cálculo de offset e flags do paginator.

## Repositórios
Testados com SQLite real (`pkg/sqlite/sqlitetest`): cada método dos contratos do recurso, parâmetro de isolamento zerado recusado, o dado de uma aplicação não aparece nem muda por outra (use `sqlitetest.Seed` para ter dois tenants) e os filtros. Os fakes ficam em `mocks/` do service, são concretos (sem generics), implementam os contratos do recurso e guardam os argumentos de cada chamada para o teste conferir que o use case passou a aplicação e o dono certos.

## Controllers
Testam via `httptest` com os fakes de `mocks/`: status por cenário (201/200/204/207/400/404/409/500), body único vs. lote (objeto vs. lista), JSON inválido, `RegisterControllers` com parâmetro nil, headers do cliente chegando em `Requester` e requisições concorrentes.

## Regras
- Asserte `Type` e `Status` do `entities.Error`, nunca texto de mensagem.
- Cada cenário monta o próprio fake, sem estado compartilhado entre `t.Run`.
- Não use `time.Sleep`. Para tempo ou aleatoriedade determinística, injete a dependência.
- Bug corrigido ganha um teste que falha sem a correção.
