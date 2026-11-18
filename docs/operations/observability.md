# Observabilidade

O Vault registra **só logs de aplicação**: o boot e os erros de função. O **log de acesso** (uma linha por requisição) é do Caddy.

## Arquivo

`vault.log`, ao lado do binário (ou `LOG_PATH`). Uma linha JSON por evento, e o arquivo guarda só as **últimas 1000 linhas** (não há rotação).

```json
{"@timestamp":"2026-11-17T12:00:00.123456789Z","log.level":"info","log.type":"boot","message":"http listening on :8083","service.runtime.version":"go1.27.4","process.pid":4242}
```

```json
{"@timestamp":"2026-11-17T12:03:10.5Z","log.level":"error","log.type":"function_error","error.origin":"internal/restful/secret/repositories/repository.go/SecretRepository.Create","error.message":"...","process.pid":4242}
```

| Campo | Significado |
|---|---|
| `@timestamp` | UTC, RFC 3339 com nanossegundos |
| `log.level` | `info` ou `error` |
| `log.type` | `boot` ou `function_error` |
| `error.origin` | `<pasta>/<arquivo>.go/<função>` de onde o erro nasceu |
| `error.message` | o erro interno (só no log, nunca na resposta) |
| `message` | texto do evento |
| `process.pid`, `service.runtime.version` | processo e versão do Go |

O arquivo é aberto com `flock` a cada linha (seguro entre processos em Unix) e tem permissão `0640`.

## O que não vai para o log

Valores de segredo, tokens, corpo de requisição. O log traz ids e nomes.

## Respostas de erro

O cliente nunca vê a mensagem interna: o `500` tem texto fixo. Para achar a causa, procure no `vault.log` o `error.origin` e o horário. O `request_id` (header `X-Request-ID`, gerado pelo Caddy) liga a requisição à linha de auditoria e ao log de acesso.

## Acompanhar

```sh
tail -f vault.log | jq .
```

Não há rota de health: para saber se o serviço está de pé, teste a porta (`APPLICATION_PORT`).
