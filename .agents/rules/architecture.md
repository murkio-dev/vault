# Regras: arquitetura

Árvore de projeto, camadas e contratos. Inegociáveis.

## Arvore de projeto:
```txt
.git/
.github/workflows/   # planejado: ainda não existe
docs/            # Documentação do serviço: README.md (índice), api.md (contrato HTTP), api/, security/, operations/, development/, guides/ e reference/. Atualizada na mesma mudança que altera rota, limite, variável ou regra
.agents/
    ├── skills/  # Pasta para Skills de inteligências artificiais
    └── rules/   # Pasta para regras inegociáveis para IA
cmd/             # Aqui deve ficar só o entrypoint, sem regras de negocio ou dominio
    ├── api/     # Somente o necessário para a api funcionar
    │   └── main.go
    ├── cli/     # Planejado (não existe ainda): somente o necessário para o cli funcionar
    ├── mcp/     # Planejado (não existe ainda): somente o necessário para o servidor mcp funcionar
    └── jobs/    # Planejado (não existe ainda): somente o necessário para os workers funcionarem
hacks/
    ├── http/ # Forma de executar requisições na api
    │   └── <nome do service>/<acao>.http # Ex: applications/create.http. Usa {{..._BACKEND}}/api/v1/<service>
    ├── make/   # Arquivos do makefile. Hoje só existe proto.mk (make proto); dev.make e env.make são planejados
    │   └── proto.mk
    └── scripts/   # Planejado: scripts de pipeline ou facilitadores (geralmente em python)
internal/
    ├── contracts/
    │   ├── usecases.go    # Contrato inegociável de UseCase (único com generics) e o alias IAccessGuard
    │   ├── transaction.go   # IAuditedTx (transação com auditoria)
    │   ├── cryptography.go  # ISecretValues (cifra)
    │   └── dependencies.go  # Dependencies: o que o initializr raiz injeta nos services
    ├── adapters/          # Implementações concretas dos contratos globais que não são de um service só: audited (IAuditedTx, em cima do sqlite.Transactor e do repository da trilha), secretvalues (ISecretValues, ponte entre o ciphertext e o secretbox) e tokenauth (o autenticador do token do SDK). Só o `initializr.go` raiz as conhece; cada uma tem `<nome>.go` e `<nome>_test.go`
    ├── config/            # Config do vault (variáveis de ambiente, .env e caminhos do banco e do log, relativos à pasta do binário): config.go (Config, New, validações) e env.go (Load, ResolvePath, BinaryDir). Só o `cmd/api/main.go` a usa
    ├── entities/          # Núcleo compartilhado por todas as camadas, inclusive pkg/: structs de domínio, erros (RFC 9457), paginator, logs de aplicação
    │   └── <nome da entity>.go
    ├── plugins/           # Planejado (não existe ainda)
    │   └── mcp/
    ├── restful/           # Camada de entrega (HTTP e gRPC). Cada service é uma pasta autocontida
    │   ├── initializr.go        # Monta o Dependencies, chama o Init de cada service (HTTP) e o InitGRPC (gRPC interno)
    │   ├── initializr_test.go   # Testes de integração do stack inteiro (HTTP e gRPC), todos neste arquivo
    │   ├── <nome do service>/   # Ex: application/
    │   │   ├── contracts/       # Contrato de repository do service (pacote <service>contracts): só `repository.go`, com uma única interface `I<Recurso>Repository` com todos os métodos do service
    │   │   ├── http_controllers/  # Entrega HTTP (pacote http_controllers): recebe a requisição, chama o IUseCase e define rotas
    │   │   │   ├── controller.go
    │   │   │   └── controller_test.go
    │   │   ├── grpc_controllers/  # Entrega gRPC (pacote grpc_controllers): mesmo molde do http_controllers, só muda o transporte. Os testes sobem o servidor com `pkgmocks.NewGRPCConn`
    │   │   │   ├── controller.go
    │   │   │   └── controller_test.go
    │   │   ├── services/        # Parse/validação de request (path id, paginação, body) -> entities.Error
    │   │   ├── useCases/        # Implementações de IUseCase (um arquivo por caso de uso + _test.go). Aceita um `errors.go` com os helpers de erro compartilhados pelos casos de uso do service (ex.: `secret/useCases/errors.go`)
    │   │   │   └── <caso_de_uso>.go
    │   │   ├── repositories/    # Implementa os contratos do service (ex: SQLite). `repository.go` e, quando precisa de funções auxiliares próprias, um arquivo de helpers (ex.: `audit/repositories/helpers.go`). O `runtime` não tem repositories (usa os dos outros services)
    │   │   │   └── repository.go
    │   │   ├── mocks/           # Só `mock.go` (sempre, em todo service, mesmo sem repository próprio, como o runtime): os fakes do service (ex.: ApplicationRepo) e os helpers (NewRepo, NewRequester, NewApp, rigs). Nunca mais de um arquivo. Helper usado por mais de um _test.go também mora aqui (ex.: o `Rig` do secret)
    │   │   └── initializr.go    # Monta repository + usecases + http_controllers do service (`Init`) e, quando o service tem gRPC, os grpc_controllers (`InitGRPC`)
    │   └── (cada novo service segue o mesmo molde)
    ├── tui/   # Planejado: camada visual de terminal
    └── workers/   # Planejado: rotinas em background
migrations/
    ├── sqlite/
    │   ├── *.up.sql
    │   └── *.down.sql
    ├── postgres/          # Planejado, não existe ainda
    │   ├── *.up.sql
    │   └── *.down.sql
    └── embed.go           # Mantém os .sql embutidos no binário para facilitar deploy
pkg/                       # Só o que não conhece o domínio do vault (infra e helpers), para não poluir o internal/. Pode abrigar código compartilhado entre projetos. Pode usar internal/entities, mas nunca importa internal/restful
    ├── sqlite/            # Abertura do banco (WAL, conexão única), DSN, IsUniqueViolation e o Transactor/Conn. sqlitetest/ dá o banco migrado dos testes
    ├── migration/         # Executor das migrations embutidas
    ├── secretbox/         # AES-256-GCM com keyring (chave ativa + antigas)
    ├── cappedfile/        # Arquivo de log com teto de linhas
    ├── graceful/          # Sobe vários servidores e os desliga juntos por sinal
    ├── authn/             # JWT do Auth (Verifier + JWKS em cache), o middleware `Authenticate` e a regra única `Identify` das duas entregas. authntest/ sobe um Auth falso
    ├── httpx/             # Entrega HTTP: respond.go (Respond/RespondList/RespondJSON/RespondError), formats.go (negociação por Accept e os renderizadores JSON, XML, YAML e tabela), bindJson.go (BindBody/BindJSON/BindIDs), request.go (PathUUID/PathMember/BindOne/Pagination), routes.go (Handle), params.go (AssertNotNilParams, usado também pelos grpc_controllers) recovery.go (panic vira 500 em RFC 9457) e server.go (o servidor HTTP pronto: `httpx.NewServer` já traz timeouts, teto de corpo e de cabeçalho, recovery e o log de boot, e implementa `graceful.Server`; os limites são constantes do pacote, como `httpx.ShutdownTimeout`)
    ├── grpcx/             # Entrega gRPC: servidor interno (só 127.0.0.1, fora do Caddy) com listener + Shutdown (server.go) e o interceptor de autenticação (authenticate.go: token do SDK vt_ só no VaultService; JWT de pessoa nos demais, pela mesma regra do HTTP em `authn.Identify`), mais os helpers dos grpc_controllers: errors.go (Error/ItemErrors), requester.go (Requester), parse.go (UUID/MemberID/Pagination) e messages.go (Page/Time/MemberMessage...). Os handlers moram em `restful/<service>/grpc_controllers/`, o `InitGRPC` que registra todos fica em `restful/initializr.go`, e o contrato `.proto` fica em `proto/` (o vault é o dono; os clientes copiam) com o código gerado versionado em `pkg/rpc/`
    ├── dependencies/      # `dependencies.New(db, box)`: monta o `contracts.Dependencies` (guard, transação auditada, cifra, repositories compartilhados) e o autenticador do token do SDK. Usada pelo `restful.Init`, pelo `cmd/api/main.go` e pelos testes. Exceção da regra de `pkg/`: conhece repositories e adapters de propósito
    ├── mocks/             # Só `mock.go`: o fake genérico de IUseCase (controllers), TokenAuthenticator, NewGRPCConn, `NewDependencies(t, db)` (Dependencies de verdade sobre um db de teste) e os geradores de dados aleatórios (GenRandom...), usados pelos testes e pelos mocks/ dos services
    └── rpc/               # Código gerado do `.proto` (versionado, nunca editado à mão)
```

### Testes sempre em par
Todo `_test.go` tem um `.go` de mesmo nome ao lado (`create.go` e `create_test.go`) e guarda tudo o que testa aquele arquivo, sem limite de tamanho. Não existe teste solto (`authorization_test.go`, `write_test.go`) nem um teste que cobre vários arquivos de uma vez: o cenário é repetido no teste de cada arquivo. Exceções que existem hoje: `migrations/migrations_test.go` (testa o conjunto de migrations embutidas, `embed.go`), `pkg/graceful/signal_unix_test.go` (o teste de sinal, só em Unix) e os `_test.go` de arquivos só de contrato, que não têm lógica para testar. Teste novo segue o par.

### Isolamento de tenant
- A aplicação (e o dono, quando o dado é do membro) entram como **parâmetros explícitos** dos métodos do contrato, depois de `Authorize` e antes de chamar o repositório. Nada de tenant ou filtro no `ctx`.
- Parâmetro de isolamento é obrigatório: uuid zero é recusado pelo repository com erro, sem tocar o banco, e nunca é lido como "sem filtro". Leitura sem dono (ex.: segredos de todos os membros) é outro método, com nome que diz isso.
- Todo repositório de tenant tem `repository_test.go` que prova: parâmetro zerado recusa, e outra aplicação não enxerga nem altera o dado.

### Onde moram as peças compartilhadas
- **Autorização**: `Authorize` é um use case do service `member` (`useCases/authorize.go`, `IUseCase[entities.AuthorizeInput, entities.AuthorizeOutput]`), injetado pelo `initializr.go` raiz nos demais services (o contrato é `contracts.IAccessGuard`). A decisão de permissão em si é `entities.Decide`, pura e testada em matriz; nenhum use case compara papel por conta própria.
- **Auditoria**: `contracts.IAuditedTx.Run` roda a mudança e grava a linha da trilha na mesma transação (fail-closed). A implementação é `internal/adapters/audited`, em cima do `sqlite.Transactor`.
- **Infra em `pkg/`** (não conhece o domínio): `authn`, `httpx`, `grpcx`, `secretbox`, `sqlite` (WAL e `Transactor`), `migration`, `cappedfile`, `graceful`. O que implementa um contrato do domínio não mora em `pkg/`: vai para `internal/adapters/` (audited, secretvalues, tokenauth) ou para o `repositories/` do service dono (a outbox é o `OutboxRepository` do invitation).
- **Não** existe `internal/middlewares/` (ver `production.md`): autenticação (`pkg/authn`) e recovery (`pkg/httpx`) vivem em `pkg/`.
- Os parsers de request ficam em `services/` de cada service, usando os helpers de `pkg/httpx` (HTTP) e `pkg/grpcx` (gRPC).

Regras de dependência: `http_controllers`/`grpc_controllers -> useCases -> contracts/entities`, `repositories -> contracts/entities`. `restful/<service>/contracts` depende só de `entities` e pode ser importado por qualquer pacote (use case de outro service, `pkg/`, `internal/contracts`). `internal/entities` pode ser importada por qualquer pacote (http_controllers, grpc_controllers, useCases, repositories, services, pkg, cmd). Só `initializr.go` conhece as implementações concretas. `internal/adapters -> contracts/entities/pkg`, e `pkg/` nunca importa `internal/restful` nem `internal/adapters`.

## contracts

`usecases.go` é o contrato absoluto para as lógicas de negócio. Nenhum caso de uso deve sobrepor esse contrato. O `Perform` **não retorna `error`**: como as operações são em lote, cada use case devolve um `Output` que carrega `Data` e `Errors` (ou `Error` quando é item único), e o controller entrega isso para `httpx.Respond`/`httpx.RespondList`, que decide o status (200/201/204, 207 em sucesso parcial, ou o status do primeiro erro).

```go
// internal/contracts/usecases.go

import "context"

type IUseCase[i any, o any] interface {
    Perform(ctx context.Context, input i) o
}
```

Convenção dos DTOs de use case: `<Nome>Input` (sempre com `Requester entities.ApiHeader`) e `<Nome>Output` com `Data` e `Errors []entities.Error` (batch) ou `Error *entities.Error` (item único/listagem).

Repositories **não usam generics**. O contrato de repository de cada service mora em `restful/<service>/contracts/repository.go`, na pasta do service dono do recurso, e é **uma única interface** `I<Recurso>Repository` com todos os métodos que o service usa (`Create`, `Exist`, `FindById`...), com os tipos de `entities` na assinatura. O pacote se chama `<service>contracts` (`applicationcontracts`) e é importado com esse alias. Use case, service e `pkg/` recebem essa interface (um campo `Repository` no use case; `Members`, `Audit`, `Secrets`, `Versions`... quando é o repository de outro service, ex.: `membercontracts.IMemberRepository` no use case de application). Não existe uma interface por método nem por grupo.

Quando dois métodos do mesmo service têm o mesmo nome e assinaturas diferentes (ex.: `List` de segredos e de versões), cada recurso tem a sua interface no mesmo arquivo (`ISecretRepository`, `IVersionRepository`).

```go
// internal/restful/application/contracts/repository.go
package applicationcontracts

import (
    "context"
    "uuid"

    "github.com/murkio-dev/<service>/internal/entities"
)

type IApplicationRepository interface {
    Create(ctx context.Context, application entities.Application) (entities.Application, error)
    Exist(ctx context.Context, application entities.Application) (bool, error)
    FindById(ctx context.Context, id uuid.UUID) (entities.Application, error)
    List(ctx context.Context, limit, offset int) ([]entities.Application, int, error)
    Update(ctx context.Context, application entities.Application) (entities.Application, error)  // Substituição completa (PUT)
    Upgrade(ctx context.Context, application entities.Application) (entities.Application, error) // Atualização parcial (PATCH)
    Delete(ctx context.Context, id uuid.UUID) error
}
```

```go
// internal/restful/application/useCases/delete.go

type DeleteApplicationUseCase struct {
    Repository applicationcontracts.IApplicationRepository
    Guard      contracts.IAccessGuard
    Tx         contracts.IAuditedTx
}
```

- **Nome**: `I<Recurso>Repository` (`IApplicationRepository`, `ISecretRepository`, `IVersionRepository`).
- **Só existe o que o recurso tem**: nenhum contrato declara método que o recurso não implementa, e não há "operação não suportada".
- **Vocabulário dos métodos**: `Create`, `Exist`, `FindById`, `List`, `Update` (substituição completa, PUT), `Upgrade` (atualização parcial, PATCH) e `Delete`. Operação que não é CRUD puro (revogar, aceitar, marcar como lida, buscar por hash) é um método da interface com o nome da intenção; ela não se disfarça de `Delete`, `Upgrade` ou `List` com filtro escondido. Operação que precisa de atomicidade entre passos é um único método do contrato, e a transação fica dentro do repository.
- **Dado que muda o resultado é parâmetro explícito do método** (aplicação, dono, filtro, hash), nunca valor escondido no `ctx`. O `ctx` leva só cancelamento, prazo e a identidade da requisição posta na borda.
- **Registro não encontrado** devolve `sql.ErrNoRows`.
- **O repository** (`repositories/repository.go`) implementa todos os contratos do recurso e declara `var _ applicationcontracts.IApplicationRepository = ApplicationRepository{}`. O `initializr.go` injeta a mesma instância em cada use case. Use case, service e `pkg/` recebem o contrato, nunca o tipo concreto.
- **Precisa de uma operação nova?** Acrescente o método à interface em `contracts/repository.go` e implemente no repository e no fake. Nunca um tipo genérico para "servir todos os recursos".
- **Generics** ficam restritos a `IUseCase[i, o]` e aos helpers de `pkg/`. Repository, contrato de repository e fake de repository não têm parâmetro de tipo.

Além dos contratos de repository (que ficam nos services), `internal/contracts/` guarda, um arquivo por classificação, `IUseCase` e o alias `IAccessGuard` (`usecases.go`), `IAuditedTx` (`transaction.go`) e `ISecretValues` (`cryptography.go`). Tipos de filtro de consulta de uma tabela (ex.: `AuditFilter`) moram em `entities/`, junto do domínio que filtram. Repository é sempre de service: a outbox, por exemplo, é o `IOutboxRepository` em `invitation/contracts/repository.go`. O `Dependencies` (`dependencies.go`) é o que o initializr raiz injeta nos services; guarda esses contratos específicos, não tipos genéricos.
