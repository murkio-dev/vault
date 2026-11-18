# Runtime

A rota que o SDK usa para ler o ambiente. É a **única** que aceita o token `sk_vt_` (e a única que não aceita JWT).

```http
GET /api/v1/runtime/{uri_id}/env?names=DATABASE_URL,JWT_KEY
Authorization: Bearer sk_vt_...
```

```json
{"data": {"uri_id": "3f9c0a7e1b2d4c5e8a6b7c8d",
          "variables": [
            {"name": "DATABASE_URL", "type": "database_uri", "version": 2, "value": "postgres://..."},
            {"name": "JWT_KEY", "type": "jwt_key", "version": 1, "value": "..."}
          ]}}
```

- Resposta **sempre JSON**, com `Cache-Control: no-store`.
- `names` (opcional) filtra as chaves, separadas por vírgula e **sem trim**, no máximo 100 (`400` acima disso). Sem `names`, devolve todas as variáveis do membro. Um nome que não existe é omitido (não é erro).
- Até 100 variáveis, ordenadas por nome.
- Referências são resolvidas para o valor atual do alvo. Uma referência cujo alvo sumiu é omitida em silêncio.
- Qualquer outro erro (uma falha de decifragem, por exemplo) é `500` para a chamada inteira.
- A leitura é auditada: `env.read` (alvo: o membro).

## O `uri_id` precisa ser o do token

O token só lê a tabela do **próprio** membro. Se o `uri_id` do path não for o do token, a resposta é `404` e o Vault grava `access.denied` (`action: env.read`, `reason: uri mismatch`).

## Efeito imediato

Token revogado ou expirado, membro suspenso ou removido e aplicação apagada dão `401` na próxima chamada.

## Rate limit

O Vault não limita taxa. No Caddy, a rota `/runtime` tem limite por token (60/min) e por IP (120/min). Veja [Deploy](../operations/deployment.md).

## gRPC

O mesmo caso de uso existe como `VaultService.ReadEnv` ([gRPC interno](grpc.md)).

Arquivo de teste: [`hacks/http/runtime/env.http`](../../hacks/http/runtime/env.http).
