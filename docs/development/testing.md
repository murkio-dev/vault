# Testes

```sh
go test -race ./...
```

Todo código novo vem com testes, no mesmo diretório (`<arquivo>_test.go`), em **pacote externo** (`package useCases_test`), com `testify` (`assert` e `require`, só em `_test.go`, então não vai para o binário). Meta: 100% de cobertura nos use cases.

## Pares

Todo `_test.go` tem um `.go` de mesmo nome ao lado (`create.go` e `create_test.go`) e guarda tudo o que testa aquele arquivo. Não existe teste solto nem teste que cobre vários arquivos: o cenário é repetido em cada um.

## Estrutura de um teste

```go
// Main test
func TestCreate(t *testing.T) {
    t.Run("Quando todos os itens são válidos", func(t *testing.T) { whenAllValid(t) })
    t.Run("Quando o repositório falha", func(t *testing.T) { whenRepositoryFails(t) })
}

// Functions
func whenAllValid(t *testing.T) { /* monta o próprio fake e confere Data, Errors e chamadas */ }
```

- Uma função `Test<Nome>` por unidade, um `t.Run("Quando ...")` por cenário, em português, cada um chamando `when<Cenario>(t)`.
- Dados vêm de `mocks.NewRequester()` e `mocks.NewApp()`, que usam os geradores `pkgmocks.GenRandom...`.
- Cada cenário monta o **próprio fake**, sem estado compartilhado entre `t.Run`.
- Assere `Type` e `Status` do `entities.Error`, nunca o texto da mensagem.
- Nada de `time.Sleep`: para tempo ou aleatoriedade, injete a dependência.
- Bug corrigido ganha um teste que falha sem a correção.

## Cenários de cada use case

1. **Build**: o construtor injeta os repositories (aceita nil sem panic).
2. **Sucesso total**: item único e lote com dados válidos aleatórios.
3. **Sucesso parcial**: lote misto (válido, inválido, não encontrado/conflito, erro de banco), um erro por item, na ordem.
4. **Não funciona**: dados inválidos; `Data` nil, `Type`/`Status` certos, repository **não** chamado.
5. **Falhas do repository**: `sql.ErrNoRows` vira `404` e outro erro vira `500`, sem vazar a mensagem.
6. **Entrada vazia**: sem `Data` nem `Errors`.
7. **Id duplicado**: reportado uma vez, nenhuma ocorrência processada.
8. **Listagem**: padrões de `page`/`limit`, limite máximo, offset e flags.

## Por camada

| Camada | Como |
|---|---|
| use cases | fakes de `mocks/mock.go` do service; guardam os argumentos para provar que a aplicação e o dono certos foram passados |
| repositories | **SQLite real** (`pkg/sqlite/sqlitetest`): cada método, parâmetro de isolamento zerado recusado, outra aplicação não vê nem altera o dado (`sqlitetest.Seed`), e cada método com o banco fechado (o erro sobe e não vira "não encontrado") |
| http_controllers | `httptest` com os fakes: status por cenário (201/200/204/207/400/404/409/500), objeto vs. lista, JSON inválido, parâmetro nil |
| grpc_controllers | sobem o servidor com `pkgmocks.NewGRPCConn` |
| initializr | `Init` registra as rotas, `InitGRPC` registra o serviço, os dois dão erro com nil. `Dependencies` vem de `pkgmocks.NewDependencies(t, db)`. `restful/initializr_test.go` cobre o stack inteiro |
| autenticação | `authntest` sobe um Auth falso (JWKS) |
| política | `entities.Decide` é testada em matriz (`policy_test.go`) |

## Mocks

Só `mocks/mock.go` por service (`// Mocks`, depois `// Helpers`), concreto, sem generics, implementando o contrato do service. Helper usado por mais de um `_test.go` também mora aqui. Nunca duplique um fake dentro de um `_test.go`.
