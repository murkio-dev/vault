# Regras: dependências

## Regra principal
**Não importamos pacotes de terceiros.** Tudo é criado aqui, dentro do projeto, usando apenas a biblioteca padrão do Go (e os pacotes `pkg/` do próprio vault). O pacote `uuid` já faz parte da stdlib do Go 1.27.

## Por quê
- **Menor binário possível.** O vault roda como um binário único em FreeBSD (4 GB de RAM, 2 núcleos), então cada dependência custa tamanho e tempo de build.
- **Segurança.** Menos código de terceiros significa menos superfície de ataque, menos CVEs herdadas e nenhum risco de supply chain.
- **Qualidade.** Código próprio é código que entendemos, testamos e mantemos por inteiro.

## Como aplicar
- Antes de pensar em uma lib, resolva com a stdlib. Se a stdlib não resolver, implemente em `pkg/<nome>/` e cubra com testes.
- Não adicione `require` novo no `go.mod`. Se achar que é inevitável, pare e peça aprovação explícita antes. IA não decide isso sozinha.
- A implementação própria não pode abrir mão de segurança nem de qualidade: validar entrada, tratar todos os erros, sem `panic` em fluxo normal, com testes (`testing.md`) e sem copiar trechos de bibliotecas sem entender o que fazem.
- Testes usam apenas o pacote `testing` da stdlib.
- Ferramentas de desenvolvimento que não entram no binário (ex.: `gofmt`, `go vet`) são permitidas.

## Exceções aprovadas
Só estas dependências de terceiros são permitidas. Qualquer outra exige aprovação explícita do dono do projeto.

| Dependência | Uso | Por que é exceção |
|---|---|---|
| `modernc.org/sqlite` | driver do `database/sql` em `pkg/sqlite` | a stdlib não tem driver de SQLite. É Go puro (sem CGO), então compila para FreeBSD em binário único |
| `go.yaml.in/yaml/v2` | decodificar e renderizar YAML em `pkg/utils` | não há alternativa na stdlib |
| `github.com/stretchr/testify` | `assert` nos testes | só entra em arquivos `_test.go`, então **não vai para o binário** |

Dentro das exceções, use só o necessário (por exemplo, `testify/assert`, sem `mock` e `suite`). Dependências indiretas que vierem delas são aceitas, mas não use diretamente.
