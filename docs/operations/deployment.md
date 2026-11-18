# Deploy

## Alvo

| | |
|---|---|
| SO | FreeBSD |
| RAM | 4 GB |
| Disco | 1 TB |
| CPU | 2 núcleos |
| Docker/Podman | **não** (binário único) |

```sh
GOOS=freebsd GOARCH=amd64 go build -o vault-api ./cmd/api
```

O binário é estático, e o SQLite é Go puro. Copie o binário para o servidor.

## Configuração em produção

A aplicação segue o 12-Factor: **defina as variáveis no ambiente do processo** (rc.d, `daemon(8)` ou o que você usar). O `.env` ao lado do binário só entra como fallback e nunca sobrescreve o ambiente. Lista em [Configuração](configuration.md).

O banco e o log ficam na pasta do binário, então escolha uma pasta com disco e backup:

```text
/usr/local/vault/
  vault-api
  vault.db       (+ -wal, -shm)
  vault.log
```

## Rede

```text
cliente ─▶ Cloudflare ─▶ Caddy (80/443) ─▶ vault HTTP  :APPLICATION_PORT
                                           vault gRPC  127.0.0.1:GRPC_PORT  (fora do Caddy)
```

- O HTTP do Vault escuta em **todas as interfaces** (`:APPLICATION_PORT`). Feche a porta no firewall para que só o Caddy a alcance: o IP do cliente que vai para a auditoria vem de headers que só o Caddy deve definir.
- O gRPC escuta só em `127.0.0.1` e **não** passa pelo Caddy.
- Portas padrão: HTTP `8083`, gRPC `50052`.

## Borda (Caddy)

Quem define TLS, rate limit, log de acesso, teto de corpo, timeouts e headers de segurança é o Caddy (`infra/Caddyfile`). O contrato público é `<host>/v1/vault/...`, e o Caddy troca o prefixo para `/api/v1`. Em produção o host é `api.murkio.com`, sem subdomínio por aplicação.

Limites do Vault no Caddyfile (sobre o teto geral por IP):

| Zona | Rota | Limite |
|---|---|---|
| `runtime_token` | `GET /v1/vault/runtime/*`, chave: header `Authorization` | 60/min |
| `runtime_ip` | `GET /v1/vault/runtime/*`, chave: IP | 120/min |
| `reveal` | `POST …/secrets/*/reveal`, chave: IP | 30/min |
| `writes` | `POST/PUT/PATCH/DELETE` em `applications` e `invitations`, chave: IP | 120/min |
| `accept_invitation` | `POST /v1/vault/invitations/accept`, chave: IP | 10/min |

Qualquer caminho fora de `/v1/vault/*` responde `404`. Teto do corpo: 64 KB. Veja o guia [Publicar atrás do Caddy](../guides/deploy-behind-caddy.md) e `infra/README.md`.

## Servidor HTTP

O Vault traz limites de proteção do protocolo: `ReadHeaderTimeout` 5 s, `ReadTimeout` 15 s, `WriteTimeout` 15 s, `IdleTimeout` 60 s, cabeçalho de até 1 MiB e corpo de até 64 KiB. Pânico em handler vira `500` (RFC 9457) e nunca derruba o processo.

## Parada

`SIGTERM` ou `Ctrl+C` desliga HTTP e gRPC juntos, com até 20 s para terminar o que está em andamento.

## Checklist

- [ ] `ENCRYPTION_KEY` gerada com `openssl rand -base64 32` e guardada à parte do banco.
- [ ] `AUTH_URL` em `https` e `AUTH_APPLICATION_ID` correto.
- [ ] `APPLICATION_BACKEND` com a URL pública.
- [ ] Porta HTTP acessível só pelo Caddy.
- [ ] Backup do `vault.db` agendado.
- [ ] `vault.log` acompanhado.
