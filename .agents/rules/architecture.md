# Regras: arquitetura

Árvore de projeto, camadas e contratos. Inegociáveis.

## Arvore de projeto:
```txt
.git/
.github/workflows/
.agents/
    ├── skills/  # Pasta para Skills de inteligências artificiais
    └── rules/   # Pasta para regras inegociáveis para IA
cmd/             # Aqui deve ficar só o entrypoint, sem regras de negocio ou dominio
    ├── api/     # Somente o necessário para a api funcionar
    │   └── main.go
    ├── cli/     # Somente o necessário para o cli funcionar
    ├── mcp/     # Somente o necessário para o servidor mcp funcionar
    └── jobs/    # Somente o necessário para os workers funcionarem
hacks/
    ├── http/ # Forma de executar requisições na api
    │   └── <nome do service>/<acao>.http # Ex: applications/create.http. Usa {{..._BACKEND}}/api/v1/<service>
    ├── make/   # Arquivos do makefile
    │   ├── dev.make
    │   └── env.make
    └── scripts/   # scripts de pipeline ou facilitadores (geralmente em python)
internal/
    ├── contracts/
    │   ├── usecases.go    # Contrato inegociável de UseCase (único com generics)
    │   └── repository.go  # TODOS os contratos de repository do serviço, num arquivo só: um por grupo de métodos, sem generics
    ├── entities/          # Núcleo compartilhado por todas as camadas, inclusive pkg/: structs de domínio, erros (RFC 9457), paginator, logs de aplicação
    │   └── <nome da entity>.go
    ├── plugins/
    │   └── mcp/
    ├── rpc/               # Camada de entrega gRPC interna (só 127.0.0.1, fora do Caddy): interceptor de autenticação (token do SDK vt_), controller que chama o IUseCase e initializr. Os contratos `.proto` ficam em `../proto` e o código gerado é versionado em `pkg/rpc/`
    ├── restful/           # Camada de entrega HTTP. Cada service é uma pasta autocontida
    │   ├── <nome do service>/   # Ex: application/
    │   │   ├── controllers/     # Recebe requisição, chama o IUseCase e define rotas
    │   │   │   ├── controller.go
    │   │   │   └── controller_test.go
    │   │   ├── services/        # Parse/validação de request (path id, paginação, body) -> entities.Error
    │   │   ├── useCases/        # Implementações de IUseCase (um arquivo por caso de uso + _test.go)
    │   │   │   └── <caso_de_uso>.go
    │   │   ├── repositories/    # Implementa os contratos do recurso (ex: SQLite)
    │   │   │   └── repository.go
    │   │   ├── mocks/           # Fakes compartilhados pelos testes (repository.go: ApplicationRepo, usecases.go: fakes de IUseCase, helpers NewRequester/NewApp)
    │   │   └── initializr.go    # Monta repository + usecases + controllers do service
    │   └── (cada novo service segue o mesmo molde)
    ├── tui/   # Camada visual de terminal
    └── workers/   # Rotinas em background
migrations/
    ├── sqlite/
    │   ├── *.up.sql
    │   └── *.down.sql
    ├── postgres/          # Preparado para o futuro
    │   ├── *.up.sql
    │   └── *.down.sql
    └── embed.go           # Mantém os .sql embutidos no binário para facilitar deploy
pkg/                       # Pacotes de apoio, para não poluir o internal/ (infra, helpers). Também pode abrigar código compartilhado entre projetos no futuro. Pode usar internal/entities
    ├── dotenv/            # Usado apenas em ambiente de desenvolvimento
    ├── migration/
    ├── sqlite/
    └── utils/             # http.go (Respond/RespondError/negociação de formato), bindJson.go (BindBody/BindIDs), gen.go (dados aleatórios)
```

### Isolamento de tenant
- A aplicação (e o dono, quando o dado é do membro) entram como **parâmetros explícitos** dos métodos do contrato, depois de `Authorize` e antes de chamar o repositório. Nada de tenant ou filtro no `ctx`.
- Parâmetro de isolamento é obrigatório: uuid zero é recusado pelo repository com erro, sem tocar o banco, e nunca é lido como "sem filtro". Leitura sem dono (ex.: segredos de todos os membros) é outro método, com nome que diz isso.
- Todo repositório de tenant tem `repository_test.go` que prova: parâmetro zerado recusa, e outra aplicação não enxerga nem altera o dado.

### Onde moram as peças compartilhadas
- **Autorização**: `Authorize` é um use case do service `member` (`useCases/authorize.go`, `IUseCase[entities.AuthorizeInput, entities.AuthorizeOutput]`), injetado pelo `initializr.go` raiz nos demais services (o contrato é `contracts.IAccessGuard`). A decisão de permissão em si é `entities.Decide`, pura e testada em matriz; nenhum use case compara papel por conta própria.
- **Auditoria**: `contracts.IAuditedTx.Run` roda a mudança e grava a linha da trilha na mesma transação (fail-closed). A implementação é `pkg/audited`, em cima do `sqlite.Transactor`.
- **Infra em `pkg/`**: `authn` (JWT do Auth e o middleware `Authenticate`), `recovery`, `audited`, `outbox`, `secretbox`, `secretvalues`, `tokenauth`, `sqlite` (WAL e `Transactor`), `utils` (resposta e parse de request).
- **Não** existe `internal/middlewares/` (ver `production.md`): autenticação e recovery vivem em `pkg/`.
- Os parsers de request ficam em `services/` de cada service, usando os helpers de `pkg/utils`.

Regras de dependência: `controllers -> useCases -> contracts/entities`, `repositories -> contracts/entities`. `internal/entities` pode ser importada por qualquer pacote (controllers, useCases, repositories, services, pkg, cmd). Só `initializr.go` conhece as implementações concretas.

## contracts

`usecases.go` é o contrato absoluto para as lógicas de negócio. Nenhum caso de uso deve sobrepor esse contrato. O `Perform` **não retorna `error`**: como as operações são em lote, cada use case devolve um `Output` que carrega `Data` e `Errors` (ou `Error` quando é item único), e o controller entrega isso para `utils.Respond`/`utils.RespondList`, que decide o status (200/201/204, 207 em sucesso parcial, ou o status do primeiro erro).

```go
// internal/contracts/usecases.go

import "context"

type IUseCase[i any, o any] interface {
    Perform(ctx context.Context, input i) o
}
```

Convenção dos DTOs de use case: `<Nome>Input` (sempre com `Requester entities.ApiHeader`) e `<Nome>Output` com `Data` e `Errors []entities.Error` (batch) ou `Error *entities.Error` (item único/listagem).

Repositories **não usam generics**. Todos os contratos de repository ficam num único arquivo, `internal/contracts/repository.go`. Cada grupo de métodos de um recurso tem o seu contrato: uma interface concreta e pequena, com os tipos de `entities` na assinatura. Quem consome importa de `contracts/` só os grupos de que precisa, podendo pegar mais de um, inclusive de recursos diferentes.

```go
// internal/contracts/repository.go

import (
    "context"
    "uuid"

    "github.com/murkio-dev/<service>/internal/entities"
)

type IApplicationCreator interface {
    Create(ctx context.Context, input entities.Application) (entities.Application, error)
}

type IApplicationReader interface {
    Exist(ctx context.Context, input entities.Application) (bool, error)
    FindById(ctx context.Context, id uuid.UUID) (entities.Application, error)
    List(ctx context.Context, limit, offset int) ([]entities.Application, int, error)
}

type IApplicationUpdater interface {
    Update(ctx context.Context, input entities.Application) (entities.Application, error)  // Substituição completa (PUT)
    Upgrade(ctx context.Context, input entities.Application) (entities.Application, error) // Atualização parcial (PATCH)
}

type IApplicationDeleter interface {
    Delete(ctx context.Context, id uuid.UUID) error
}
```

```go
// internal/restful/application/useCases/delete.go: injeta só o que usa, de mais de um contrato

type DeleteUseCase struct {
    Finder  contracts.IApplicationReader
    Deleter contracts.IApplicationDeleter
}
```

- **Nome**: `I<Recurso><Grupo>` (`IApplicationReader`, `ISecretCreator`). Um grupo é um conjunto coeso de métodos que o consumidor usa junto. Não é uma interface por método à força, nem uma interface gorda com o recurso inteiro.
- **Só existe o que o recurso tem**: nenhum contrato declara método que o recurso não implementa, e não há "operação não suportada".
- **Vocabulário dos métodos**: `Create`, `Exist`, `FindById`, `List`, `Update` (substituição completa, PUT), `Upgrade` (atualização parcial, PATCH) e `Delete`. Operação que não é CRUD puro (revogar, aceitar, marcar como lida, buscar por hash) é um método com o nome da intenção, num contrato do grupo; ela não se disfarça de `Delete`, `Upgrade` ou `List` com filtro escondido. Operação que precisa de atomicidade entre passos é um único método do contrato, e a transação fica dentro do repository.
- **Dado que muda o resultado é parâmetro explícito do método** (aplicação, dono, filtro, hash), nunca valor escondido no `ctx`. O `ctx` leva só cancelamento, prazo e a identidade da requisição posta na borda.
- **Registro não encontrado** devolve `sql.ErrNoRows`.
- **O repository** (`repositories/repository.go`) implementa todos os contratos do recurso e declara `var _ contracts.IApplicationReader = (*Repository)(nil)` para cada um. O `initializr.go` injeta a mesma instância em cada campo. Use case, service e `pkg/` recebem o contrato, nunca o tipo concreto nem um contrato maior do que o que usam.
- **Precisa de uma operação nova?** Acrescente ao contrato do grupo ou crie outro no `repository.go`. Nunca um tipo genérico para "servir todos os recursos".
- **Generics** ficam restritos a `IUseCase[i, o]` e aos helpers de `pkg/`. Repository, contrato de repository e fake de repository não têm parâmetro de tipo.

Além dos contratos de repository, `contracts/` guarda `IAuditedTx` (transação com auditoria), `ISecretValues` (cifra) e o alias `IAccessGuard`. O `Shared` do initializr raiz guarda esses contratos específicos, não tipos genéricos.
