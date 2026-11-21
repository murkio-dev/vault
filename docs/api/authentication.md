# Autenticação

Toda rota de `/api/*` exige `Authorization: Bearer <credencial>`. Existem dois tipos de credencial, e cada rota aceita **só um**.

| Credencial | Quem usa | Onde vale |
|---|---|---|
| **JWT** da conta Murkio (emitido pelo Identity) | pessoas, painel, outros serviços | tudo em `/api/v1/*`, **exceto** `/api/v1/runtime/*` |
| **Token do SDK** `sk_vt_...` | o código da aplicação (SDK) | **só** `/api/v1/runtime/*` |

O Vault decide pelo **prefixo** (`sk_vt_`) antes de qualquer consulta. Credencial do tipo errado é recusada com `401`, sem tocar o banco.

```http
GET /api/v1/applications HTTP/1.1
Authorization: Bearer eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCIs...
```

```http
GET /api/v1/runtime/3f9c0a7e1b2d4c5e8a6b7c8d/env HTTP/1.1
Authorization: Bearer sk_vt_Zk3...
```

## JWT

O Vault valida o token **localmente**, com o JWKS do Identity, lido pelo gRPC interno (`SigningKeyService.GetJWKS` em `IDENTITY_GRPC_ADDR`, para a aplicação `AUTH_APPLICATION_ID`). Regras, nesta ordem:

1. Tamanho de até 8 KiB e três partes.
2. Header com `alg=EdDSA`, `typ=JWT` e `kid`.
3. Assinatura Ed25519 com a chave do `kid`.
4. `iss = "enterprise-auth"`, e `aud` e `aid` iguais ao `AUTH_APPLICATION_ID`.
5. `exp` presente e no futuro, `iat` não adiante, com 30 s de folga.
6. `sub` é um UUID não zero.

Qualquer falha devolve o mesmo `401`. O role `admin` no JWT faz da pessoa **platform admin**.

O JWKS fica em cache (`max-age`, entre 30 s e 1 h, padrão 5 min). Um `kid` desconhecido força uma nova busca, no máximo uma a cada 30 s. Se o Identity cair, as chaves em cache continuam valendo.

## Token do SDK

O token (`sk_vt_` + 43 caracteres) é conferido **no banco a cada requisição**:

- não pode estar revogado nem expirado;
- o membro precisa estar ativo e não removido;
- a aplicação não pode estar apagada.

Por isso revogar o token, suspender o membro ou apagar a aplicação vale **na hora**, mesmo que o JWT da pessoa ainda esteja dentro do prazo. O `last_used_at` é atualizado no máximo uma vez por minuto.

## Resposta de falha

```http
HTTP/1.1 401 Unauthorized
WWW-Authenticate: Bearer
Cache-Control: no-store
Content-Type: application/json

{"error":{"type":"https://murkio.com/en-us/docs/vault/errors/#unauthorized","title":"Unauthorized","status":401,...}}
```

O corpo é o mesmo para qualquer motivo, para não revelar o que falhou.

## gRPC

O gRPC usa a mesma regra (`authn.Identify`). Veja [gRPC interno](grpc.md).
