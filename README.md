# vault

Cofre de variáveis e segredos do Murkio, para acabar com `.env` complexo. O produto e as
decisões estão em `../schema/vault.md`; o contrato HTTP em `docs/api.md`.

Go 1.27, só `net/http` e SQLite (`modernc.org/sqlite`, Go puro). Sem Docker, Postgres,
Redis, RabbitMQ nem HashiCorp Vault.

## Estado

| Parte | Estado |
| --- | --- |
| Aplicações (CRUD, soft delete com cascata) | feito, autorizado e auditado |
| Autenticação das pessoas (JWT do Auth, JWKS) | feito (`pkg/authn`, `middlewares.Authenticate`) |
| Papéis e permissões (`entities.Decide`, `access.Guard`) | feito, com matriz testada |
| Auditoria append-only, fail-closed | feito (trigger no banco + `audit.Audited`) |
| Retenção da auditoria (1 ano) | trigger e `PurgeExpired` prontos; falta o job que os chama |
| Cifragem em repouso (`pkg/secretbox`) | pronta; a chave é validada no boot |
| Membros e convites (com outbox para o e-mail) | feito |
| Segredos (tipos, versões cifradas, referências, `reveal` auditado) | feito |
| Tokens do SDK e `GET /runtime/{uri_id}/env` | feito |
| gRPC interno (`VaultService.ReadEnv`, só 127.0.0.1) | feito |
| Leitura da auditoria (cursor, filtros) | feito |
| Geração de chaves (api key, JWT, PASETO), banco dinâmico | só desenhado |

O Caddy expõe o vault em `<host>/v1/vault/...` (`../infra/Caddyfile`), com rate limit por IP e por rota (leitura do ambiente também por token) e corpo de no máximo 64 KB. O Caddy troca o prefixo `/v1/vault` por `/api/v1`, e o vault segue recebendo `/api/v1/...`. O gRPC interno não passa por ele.

## Rodando

```sh
cp .env.example .env     # preencha AUTH_APPLICATION_ID e ENCRYPTION_KEY (openssl rand -base64 32)
go run ./cmd/api
```

O `.env`, o banco (`SQLITE_PATH`) e o log (`<binário>.log`, ou `LOG_PATH`) ficam **ao lado do binário**, de onde quer que ele seja iniciado (no `go run`, na pasta atual). O `.env` também é lido da pasta atual, e o ambiente do processo vale mais que o arquivo.

### Lendo os segredos de outro serviço

Cada serviço lê os segredos dele pelo SDK do vault: `GET /runtime/{uri_id}/env?names=A,B` com o
token `vt_` do membro dono (HTTP, só a stdlib; o gRPC interno é para chamadas de máquina no
mesmo host). O messaging já faz isso no boot (`VAULT_URL`, `VAULT_URI_ID`, `VAULT_TOKEN`, ver
`../messaging/README.md`): o valor do vault vale mais que o do `.env`, e só os campos marcados
como segredo são lidos. O auth **não** usa o vault: ele sobe sempre com o `.env`, porque o
vault valida os tokens pelo JWKS dele e a dependência seria circular.

### Ligando ao auth (dev)

O vault confia em **uma** aplicação do auth: a das contas Murkio, que é a aplicação de
plataforma `murkio`. Quem tem conta nela usa o vault, e quem tem o role `admin` nela é o
platform admin.

```sh
# 1. auth: só ENCRYPTION_KEY e APPLICATION_PORT no .env. No primeiro boot de um banco vazio
#    ele cria sozinho a plataforma e grava platform-credentials.txt (0600) ao lado do banco,
#    com application_id, admin_email, admin_password e platform_secret_key.
cd ../identity && make run-api

# 2. vault: AUTH_URL=http://localhost:8080, AUTH_APPLICATION_ID=<application_id do arquivo>,
#    ENCRYPTION_KEY e GRPC_PORT no .env
cd ../vault && go run ./cmd/api

# 3. entre como o admin (a sk_ e a senha vêm do arquivo) e use o access token no vault:
curl -s -X POST localhost:8080/v1/login -H "Authorization: Bearer $SK" \
  -H 'Content-Type: application/json' -d '{"email":"...","password":"..."}'
curl -s localhost:8083/api/v1/applications -H "Authorization: Bearer <access_token>"
```

Guarde as credenciais e apague o `platform-credentials.txt`. Esse fluxo foi conferido com os
binários reais do auth e do vault: login, criar aplicação, criar segredo, gerar token e ler o
ambiente por HTTP (`/api/v1/runtime/{uri_id}/env`) e por gRPC (`VaultService.ReadEnv`).

Sem os workers do auth (ainda não existem) nenhum e-mail sai: um cadastro novo só é
verificado lendo o código da `outbox_events` do `identity.db`. O admin do bootstrap já nasce
com o e-mail verificado.

## Estrutura

```
cmd/api/main.go                  # config, banco, migrations, servidor
internal/
  contracts/                     # IUseCase e os contratos de repository (um por grupo, todos em repository.go), de auditoria e de cifra
  entities/                      # núcleo: Actor, Decide (permissões), AuditLog, Member, erros RFC 9457
  restful/                       # um service por pasta: controllers, services, useCases, repositories, mocks, initializr
    initializr.go                # monta o que os services dividem e registra as rotas
    member/                      # membros e o use case Authorize, o porteiro de toda ação
    audit/                       # leitura da trilha de auditoria
    invitation/                  # convites por e-mail (token inv_, outbox)
    secret/                      # segredos: metadados, versões cifradas, reveal
    token/                       # tokens do SDK (vt_)
    runtime/                     # GET /runtime/{uri_id}/env
  rpc/                           # gRPC interno (só 127.0.0.1): VaultService.ReadEnv, mesmo token vt_
    application/                 # CRUD de aplicações
migrations/sqlite/               # *.up.sql (rodam no boot) e *.down.sql (nunca rodam sozinhas)
hacks/http/<service>/<ação>.http # requests de exemplo de cada rota
pkg/                             # infra: authn (+Authenticate), recovery, audited, outbox, secretbox,
                                 # secretvalues, tokenauth, sqlite (+sqlitetest), mocks (fakes únicos),
                                 # migration, dotenv, utils
```

## Contratos

`IUseCase`, `IAuditedTx` (mudança + trilha na mesma transação), `ISecretValues` (cifra) e os
contratos de repository, em `contracts/repository.go`. Eles são pequenos, concretos e sem
generics: um por grupo de métodos (`IMemberFinder`, `ISecretOwnerFinder`, `ITokenRevoker`...), e
cada use case recebe só os que usa. A aplicação, o dono, o segredo e o hash são parâmetros
explícitos do método; o repository recusa o id zerado (`entities.ErrMissingParam`) e nunca o
lê como "sem filtro". Cada repository tem teste de isolamento entre aplicações.

## Regras que o código garante

- O valor de um segredo nunca está numa entidade serializável nem numa listagem.
- Quem não é membro recebe `404` em tudo; quem é membro e não pode recebe `403`.
- Remover ou suspender um membro vale na hora: o banco é consultado a cada requisição.
- Toda mudança grava a auditoria **na mesma transação**; sem auditoria, a mudança não acontece.
- O banco recusa `UPDATE` em `audit_logs` e só deixa passar o `DELETE` do que passou de 1 ano ([retenção](docs/retention.md)). O expurgo está pronto (`PurgeExpired`); quem o chama é o job, que ainda não existe.
- Os valores ficam cifrados (AES-256-GCM, AAD por versão); token e convite só existem como hash.
- As respostas com valor (`reveal`, `runtime/env`, criação de token e de convite) são sempre JSON, ignorando o `Accept`.
- Corpo com campo desconhecido é `400`: o cliente não consegue forjar `created_by`, `member_id` ou papel.

## Testes

```sh
go test -race ./...
```

Os testes de segurança (`secrets_test.go`, `tokens_test.go`, `members_test.go`) varrem o
arquivo do banco procurando o valor em claro e provam o isolamento entre contas, a revogação
imediata e o fail-closed da auditoria. `internal/restful/restful_test.go` sobe a pilha real (autenticação, guard, transação,
auditoria, SQLite) com só o Auth falso (`pkg/authn/authntest`).
