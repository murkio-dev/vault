<h1 align="center">Vault</h1>

<p align="center">
Segredos da sua aplicação em um só lugar, no lugar do <code>.env</code>.
</p>

<p align="center">
<img src="https://img.shields.io/static/v1?label=go&message=1.27&color=00ADD8" alt="Go 1.27" />
<img src="https://img.shields.io/static/v1?label=storage&message=SQLite&color=003B57" alt="SQLite" />
<img src="https://img.shields.io/static/v1?label=target&message=FreeBSD&color=AB2B28" alt="FreeBSD" />
<img src="https://img.shields.io/static/v1?label=dependencies&message=stdlib%20first&color=success" alt="stdlib first" />
<img src="https://img.shields.io/static/v1?label=license&message=PolyForm%20NC%201.0.0&color=blue" alt="PolyForm Noncommercial 1.0.0" />
</p>

<div align="center">
  <a href="docs/README.md">Documentação</a>
  <span>&nbsp;&nbsp;•&nbsp;&nbsp;</span>
  <a href="docs/api.md">API</a>
  <span>&nbsp;&nbsp;•&nbsp;&nbsp;</span>
  <a href="docs/guides/create-token-and-read-env.md">Guias</a>
  <span>&nbsp;&nbsp;•&nbsp;&nbsp;</span>
  <a href="docs/development/architecture.md">Arquitetura</a>
  <span>&nbsp;&nbsp;•&nbsp;&nbsp;</span>
  <a href="../schema/vault.md">Desenho do produto</a>
  <br />
</div>

### [Leia a documentação →](docs/README.md)

## O que é o Vault?

O Vault é o serviço de segredos da plataforma Murkio. Em vez de espalhar `.env` por máquinas, você guarda os valores no Vault e o código lê só o que precisa, em tempo de execução, com um **token** revogável na hora.

Ele é um **binário único em Go**, com SQLite ao lado dele. Não precisa de Docker, Postgres, Redis nem RabbitMQ.

```bash
# o SDK (ou o seu código) só precisa de uma URI e de um token
curl -H "Authorization: Bearer sk_vt_..." \
  https://api.murkio.com/v1/vault/runtime/<uri_id>/env
```

```json
{
  "data": {
    "uri_id": "3f9c0a7e1b2d4c5e8a6b7c8d",
    "variables": [
      { "name": "DATABASE_URL", "type": "database_uri", "version": 3, "value": "postgres://..." }
    ]
  }
}
```

O que o Vault entrega:

- **Aplicações** com **membros** (`admin` e `developer`), convidados por e-mail com link de uso único.
- **Uma tabela de segredos por membro**, com tipos (`string`, `password`, `url`, `database_uri`, `api_key`, `jwt_key`, `paseto_key`), versões e referências entre segredos da mesma aplicação.
- **Valores cifrados em repouso** (AES-256-GCM) com chave fora do banco e rotação por chave anterior.
- **Tokens do SDK** (`sk_vt_...`): só o hash é guardado, aparecem uma única vez e param de valer no instante em que são revogados.
- **Auditoria append-only e fail-closed**: leituras e acessos negados também entram.
- **API REST** (`/api/v1`) com lote, formatos JSON, XML, YAML e tabela, erros em RFC 9457, e um **gRPC interno** para outros serviços.

## Instalar e rodar

O Vault roda em Linux, macOS e Windows para desenvolvimento, e em **FreeBSD** em produção.

> **Requisitos:** Go 1.27 e uma aplicação do Vault cadastrada no Identity (o `AUTH_APPLICATION_ID`). As pessoas entram com a conta Murkio, e o Vault valida o JWT localmente pelo JWKS do Identity.

```sh
cp .env.example .env
# gere a chave de cifragem (32 bytes em base64) e preencha ENCRYPTION_KEY e AUTH_APPLICATION_ID
openssl rand -base64 32

go run ./cmd/api              # sobe a API (HTTP em APPLICATION_PORT, gRPC em 127.0.0.1:GRPC_PORT)
go build -o bin/vault-api ./cmd/api   # binário único
go test -race ./...           # testes
```

O banco (`vault.db`) e o log (`vault.log`, últimas 1000 linhas) ficam **ao lado do binário**. Veja [Configuração](docs/operations/configuration.md).

### Atualizar

Não há comando de atualização. Troque o binário e reinicie: as migrations embutidas rodam sozinhas no boot.

Para trocar a chave de cifragem sem perder dados, siga a [Rotação da `ENCRYPTION_KEY`](docs/operations/key-rotation.md).

## Links rápidos

- Introdução
  - [O que é o Vault?](docs/introduction/what-is-vault.md)
  - [Conceitos](docs/introduction/concepts.md)
  - [Instalação](docs/introduction/installation.md)
  - [Quickstart](docs/introduction/quickstart.md)

- API
  - [Contrato HTTP completo e matriz de permissões](docs/api.md)
  - [Autenticação (JWT e token `sk_vt_`)](docs/api/authentication.md)
  - [Formatos de resposta](docs/api/formats.md)
  - [Lote e entidade única](docs/api/batch.md)
  - [Paginação](docs/api/pagination.md)
  - [Erros (RFC 9457)](docs/api/errors.md)
  - [Aplicações](docs/api/applications.md)
  - [Membros](docs/api/members.md)
  - [Convites](docs/api/invitations.md)
  - [Segredos](docs/api/secrets.md)
  - [Tokens](docs/api/tokens.md)
  - [Runtime (`/runtime/{uri_id}/env`)](docs/api/runtime.md)
  - [Auditoria](docs/api/audit-logs.md)
  - [gRPC interno](docs/api/grpc.md)

- Segurança
  - [Visão geral](docs/security/overview.md)
  - [Cifragem](docs/security/encryption.md)
  - [Permissões](docs/security/permissions.md)
  - [Reportar uma vulnerabilidade](SECURITY.md)

- Operação
  - [Configuração (`.env`)](docs/operations/configuration.md)
  - [Banco (SQLite)](docs/operations/database.md)
  - [Observabilidade](docs/operations/observability.md)
  - [Deploy (FreeBSD, Caddy)](docs/operations/deployment.md)
  - [Rotação da chave](docs/operations/key-rotation.md)
  - [Retenção](docs/operations/retention.md)

- Desenvolvimento
  - [Primeiros passos](docs/development/getting-started.md)
  - [Arquitetura](docs/development/architecture.md)
  - [Pacotes (`pkg/`)](docs/development/packages.md)
  - [Testes](docs/development/testing.md)
  - [Proto (gRPC)](docs/development/proto.md)
  - [Arquivos `.http`](docs/development/http-files.md)

- Referência
  - [Limitações atuais](docs/reference/limitations.md)
  - [Variáveis de ambiente](docs/reference/env-cheatsheet.md)

## Guias

- Aplicações e pessoas
  - [Criar uma aplicação](docs/guides/create-application.md)
  - [Convidar um membro](docs/guides/invite-member.md)
  - [Aceitar um convite](docs/guides/accept-invitation.md)

- Segredos
  - [Guardar um segredo](docs/guides/store-secret.md)
  - [Referenciar um segredo de outro membro](docs/guides/reference-secret.md)
  - [Revelar um valor](docs/guides/reveal-secret.md)

- Tokens e SDK
  - [Criar um token e ler o ambiente](docs/guides/create-token-and-read-env.md)
  - [Revogar um token](docs/guides/revoke-token.md)

- Auditoria
  - [Ler a trilha de auditoria](docs/guides/read-audit-trail.md)

- Operação
  - [Rotacionar a `ENCRYPTION_KEY`](docs/guides/rotate-encryption-key.md)
  - [Publicar atrás do Caddy](docs/guides/deploy-behind-caddy.md)

- Integração e desenvolvimento
  - [Chamar o Vault por gRPC](docs/guides/call-over-grpc.md)
  - [Criar um novo service](docs/guides/add-a-new-service.md)

## Contribuindo

Leia [Desenvolvimento > Primeiros passos](docs/development/getting-started.md) e [Arquitetura](docs/development/architecture.md). As regras do projeto ficam em [`.agents/rules/`](.agents/rules) e a skill de trabalho em [`.agents/skills/go-developer.md`](.agents/skills/go-developer.md).

Antes de abrir um PR: `gofmt -l .`, `go vet ./...` e `go test -race ./...`. Quem commita é o único autor e responde pelo commit ([`commits.md`](.agents/rules/commits.md)).

## Segurança

O código é público para ser conferido: veja [`SECURITY.md`](SECURITY.md) para reportar uma vulnerabilidade e para os comandos que mostram o que o Vault faz (e não faz) com os seus dados.

## Licença

[PolyForm Noncommercial 1.0.0](LICENSE). O código é **source-available**: você pode ler, auditar, rodar e modificar para fins não comerciais (estudo, pesquisa, uso pessoal, organizações sem fins lucrativos, entre outros, conforme o texto da licença). **Não é open source**: uso comercial exige uma licença da Murkio.
