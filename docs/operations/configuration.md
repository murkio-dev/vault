# Configuração

Toda a configuração vem de variáveis de ambiente. Há um modelo em [`.env.example`](../../.env.example).

## Onde o Vault procura

Em ordem de prioridade (a primeira que define a variável vence):

1. o **ambiente do processo**;
2. o `.env` **ao lado do binário**;
3. o `.env` do **diretório atual**.

O arquivo nunca sobrescreve uma variável já definida. Linhas em branco e `#` são ignoradas, e aspas `"` e `'` em volta do valor são removidas. Um arquivo ausente é ignorado. Um arquivo que existe mas não abre é erro.

Em produção prefira definir as variáveis no ambiente do processo ([Deploy](deployment.md)).

## Variáveis

| Variável | Padrão | Obrigatória | O que é |
|---|---|---|---|
| `APPLICATION_PORT` | | sim | Porta HTTP (1 a 65535). Escuta em todas as interfaces |
| `APPLICATION_BACKEND` | vazio | não | URL pública base. Usada para montar o `uri` devolvido ao criar um token |
| `GRPC_PORT` | `50052` | não | Porta do gRPC interno (1 a 65535, diferente da HTTP). O host é sempre `127.0.0.1` |
| `IDENTITY_GRPC_ADDR` | `127.0.0.1:50051` | não | gRPC interno do Identity, de onde vem o JWKS. `host:porta` só em loopback (`127.0.0.1`, `::1` ou `localhost`) |
| `AUTH_APPLICATION_ID` | | sim | UUID (não zero) da aplicação do Vault no Identity |
| `ENCRYPTION_KEY` | | sim | Chave ativa: base64 de exatamente 32 bytes |
| `ENCRYPTION_KEYS_PREVIOUS` | vazio | não | Chaves antigas em base64, separadas por vírgula |
| `SQLITE_PATH` | `vault.db` | não | Arquivo do banco |
| `LOG_PATH` | `vault.log` | não | Arquivo de log |
| `SQLITE_TEST_PATH` | vazio | não | Só para testes (`file::memory:?cache=shared`). Não use em produção |

Valor vazio ou só com espaços conta como não definido.

## Caminhos relativos

`SQLITE_PATH` e `LOG_PATH` relativos são resolvidos a partir da **pasta do binário**. Ficam como estão: caminho vazio, `:memory:`, URIs `file:` e caminhos absolutos. Sob `go run` (e nos testes) a base é o diretório atual.

O banco é criado com permissão `0600` e a pasta com `0750`. O log, com `0640`.

## Validações no boot

O Vault não sobe (e sai com código 1) se:

- falta uma variável obrigatória (a mensagem nomeia o campo da struct, como `Port`);
- uma porta é inválida, ou as duas portas são iguais;
- `IDENTITY_GRPC_ADDR` não é `host:porta` em loopback;
- `AUTH_APPLICATION_ID` não é um UUID ou é zero;
- `ENCRYPTION_KEY` não é base64 de 32 bytes, ou uma chave anterior é igual à ativa.

A mensagem de erro de chave nunca contém o valor.

## Exemplo

```sh
APPLICATION_PORT=8083
APPLICATION_BACKEND=http://localhost:8083
GRPC_PORT=50052

IDENTITY_GRPC_ADDR=127.0.0.1:50051
AUTH_APPLICATION_ID=0194f3a2-7c10-7000-8000-000000000001

SQLITE_PATH=vault.db
LOG_PATH=vault.log

ENCRYPTION_KEY=<openssl rand -base64 32>
ENCRYPTION_KEYS_PREVIOUS=
```

Uma tabela única em [Variáveis de ambiente](../reference/env-cheatsheet.md).
