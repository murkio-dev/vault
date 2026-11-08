# Regras: comentários

Comentário é exceção, não regra. O código deve se explicar pelos nomes e pela arquitetura; só se comenta o que é **específico e diferente do padrão da arquitetura**.

## Marcadores de seção (obrigatórios)
Só os arquivos predefinidos pela arquitetura (abaixo) usam marcadores. Eles seguem a mesma ordem de blocos, separados por comentários de uma linha, exatamente com estes nomes:

| Arquivo | Marcadores, na ordem |
|---|---|
| `useCases/<caso>.go` | `// Builder`, `// DTOS`, `// UseCase` |
| `repositories/repository.go` | `// Builder`, `// Functions` |
| `*_test.go` | `// Main test`, `// Functions` (e `// Mocks` ou `// Helpers` quando houver) |
| `mocks/*.go` | `// Mocks`, `// Helpers` |

Os marcadores não contam como "comentário explicativo": são estrutura e valem em todo arquivo novo desses tipos. Os controllers (`controller.go`) e os demais arquivos (`cmd/`, `entities/`, `contracts/`, `pkg/`, `services/`, `initializr.go`, `migrations/`) não usam marcadores.

## Quando comentar
Comente apenas funções (ou tipos) com comportamento próprio, que **não** seguem o molde padrão do service. Exemplos que pedem comentário: `utils.unwrapSingle`, `negotiateFormat`, `BindIDs` e `callerOrigin`. Exemplos que **não** pedem: `Perform` de um use case, `Create`/`Delete` de repository, handlers de controller, construtores `New...`, getters e DTOs.

## Formato do comentário
- Mínimo de **5 linhas**. Comentário menor que isso não deve existir: ou se escreve o completo, ou não se comenta.
- Vai logo acima da função, no formato godoc (começa com o nome da função).
- Deve cobrir, nesta ordem:
  1. O que a função faz e **por que** ela existe.
  2. Entradas esperadas e o que cada uma significa, incluindo casos limite.
  3. O que retorna, incluindo erros e valores vazios.
  4. **Pelo menos um exemplo** de entrada e saída.
  5. Efeitos colaterais, regras ou pegadinhas (concorrência, ordem, formato).
- Sem comentário que repete o código, sem `TODO` solto e sem código comentado. `TODO` só com contexto de quem/o quê depende.

Exemplo:
```go
// unwrapSingle remove o wrapper de lista quando a resposta tem uma única
// entidade, para que o cliente receba um objeto em vez de um array de um item.
// Recebe o output do use case já pronto para serialização e devolve o mesmo
// valor se não houver nada a desembrulhar (formato inesperado ou erro de normalize).
// Exemplo: {"data":[{"id":1}]} vira {"data":{"id":1}}, e {"errors":[{...}]} vira
// {"error":{...}}. Respostas mistas (data + errors) e lotes reais (2+ itens)
// mantêm as listas. Não é aplicada em listagens (veja RespondList).
func unwrapSingle(output any) any {
```
