# Documentação do Vault

O Vault guarda os segredos de uma aplicação (as variáveis que antes ficavam em `.env`) e os entrega ao código em tempo de execução, por uma URI estável e um token revogável. É só API: HTTP público (atrás do Caddy) e gRPC interno.

O lugar do serviço na plataforma está em [`../../schema/vault.md`](../../schema/vault.md) e [`../../schema/architecture.md`](../../schema/architecture.md).

## Por onde começar

| Quero… | Leia |
|---|---|
| entender o que o Vault faz | [O que é o Vault?](introduction/what-is-vault.md) e [Conceitos](introduction/concepts.md) |
| rodar localmente | [Instalação](introduction/installation.md) e [Quickstart](introduction/quickstart.md) |
| chamar a API | [Contrato HTTP](api.md) |
| ler o ambiente no meu código (SDK) | [Criar um token e ler o ambiente](guides/create-token-and-read-env.md) e [Runtime](api/runtime.md) |
| configurar ou operar o serviço | [Configuração](operations/configuration.md) e [Deploy](operations/deployment.md) |
| saber o que é seguro e por quê | [Segurança](security/overview.md) |
| mexer no código | [Primeiros passos](development/getting-started.md) e [Arquitetura](development/architecture.md) |
| saber o que ainda não é como se espera | [Limitações atuais](reference/limitations.md) |

## Introdução

- [O que é o Vault?](introduction/what-is-vault.md)
- [Conceitos](introduction/concepts.md): aplicação, membro, papel, URI, segredo, token.
- [Instalação](introduction/installation.md)
- [Quickstart](introduction/quickstart.md)

## API

- [Contrato HTTP](api.md): todas as rotas, a matriz de permissões e os erros.
- [Autenticação](api/authentication.md)
- [Formatos](api/formats.md)
- [Lote](api/batch.md)
- [Paginação](api/pagination.md)
- [Erros](api/errors.md)
- [Aplicações](api/applications.md), [Membros](api/members.md), [Convites](api/invitations.md), [Segredos](api/secrets.md), [Chaves de API](api/api-keys.md), [Runtime](api/runtime.md), [Auditoria](api/audit-logs.md)
- [gRPC interno](api/grpc.md)

## Segurança

- [Visão geral](security/overview.md)
- [Cifragem](security/encryption.md)
- [Permissões](security/permissions.md)

## Operação

- [Configuração (`.env`)](operations/configuration.md)
- [Banco (SQLite)](operations/database.md)
- [Observabilidade](operations/observability.md)
- [Deploy](operations/deployment.md)
- [Rotação da `ENCRYPTION_KEY`](operations/key-rotation.md)
- [Retenção](operations/retention.md)

## Desenvolvimento

- [Primeiros passos](development/getting-started.md)
- [Arquitetura](development/architecture.md)
- [Pacotes (`pkg/`)](development/packages.md)
- [Testes](development/testing.md)
- [Proto (gRPC)](development/proto.md)
- [Arquivos `.http`](development/http-files.md)

## Guias

- [Criar uma aplicação](guides/create-application.md)
- [Convidar um membro](guides/invite-member.md)
- [Aceitar um convite](guides/accept-invitation.md)
- [Guardar um segredo](guides/store-secret.md)
- [Referenciar um segredo](guides/reference-secret.md)
- [Revelar um valor](guides/reveal-secret.md)
- [Criar um token e ler o ambiente](guides/create-token-and-read-env.md)
- [Revogar um token](guides/revoke-token.md)
- [Ler a trilha de auditoria](guides/read-audit-trail.md)
- [Rotacionar a `ENCRYPTION_KEY`](guides/rotate-encryption-key.md)
- [Publicar atrás do Caddy](guides/deploy-behind-caddy.md)
- [Chamar o Vault por gRPC](guides/call-over-grpc.md)
- [Criar um novo service](guides/add-a-new-service.md)

## Referência

- [Limitações atuais](reference/limitations.md)
- [Variáveis de ambiente](reference/env-cheatsheet.md)
