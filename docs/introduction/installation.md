# Instalação

O Vault é um binário único. Não há pacote, imagem Docker nem instalador: você compila e executa.

## Requisitos

- Go 1.27.
- Uma aplicação do Vault cadastrada no Identity. O id dela é o `AUTH_APPLICATION_ID`.
- O Identity na mesma máquina, com o gRPC interno em `IDENTITY_GRPC_ADDR`, para o Vault buscar o JWKS.
- Uma chave de cifragem de 32 bytes em base64.

Produção: FreeBSD (4 GB de RAM, 2 núcleos), sem Docker. Desenvolvimento: Linux (Debian/WSL) ou Windows.

## Compilar

```sh
go build -o bin/vault-api ./cmd/api
```

O binário é estático (SQLite em Go puro, sem CGO). Para FreeBSD:

```sh
GOOS=freebsd GOARCH=amd64 go build -o bin/vault-api ./cmd/api
```

## Configurar

```sh
cp .env.example .env
openssl rand -base64 32      # cole em ENCRYPTION_KEY
```

Preencha também `AUTH_APPLICATION_ID`. As demais variáveis têm padrão. A lista completa está em [Configuração](../operations/configuration.md).

O `.env` é lido de **ao lado do binário** e, depois, do diretório atual. Variáveis já definidas no ambiente do processo nunca são sobrescritas pelo arquivo.

## Rodar

```sh
go run ./cmd/api        # desenvolvimento
./bin/vault-api         # binário
```

No primeiro boot o Vault:

1. lê a configuração e abre o log (`vault.log`);
2. cria o banco (`vault.db`) e aplica as migrations `0001` a `0008`;
3. valida a `ENCRYPTION_KEY`;
4. sobe o gRPC em `127.0.0.1:GRPC_PORT`;
5. sobe o HTTP em `:APPLICATION_PORT`.

Para encerrar, `SIGTERM` ou `Ctrl+C`: os dois servidores desligam juntos, com até 20 segundos para terminar as requisições em andamento.

## Testar

```sh
gofmt -l .
go vet ./...
go test -race ./...
```

Próximo: [Quickstart](quickstart.md).
