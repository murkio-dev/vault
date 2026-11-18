# O que é o Vault?

O Vault elimina a necessidade de `.env` complexos. O código recebe uma **URI** (público) e um **token** (secreto) e lê os valores direto do Vault:

```bash
curl -H "Authorization: Bearer sk_vt_..." \
  "https://api.murkio.com/v1/vault/runtime/<uri_id>/env?names=DATABASE_URL,JWT_KEY"
```

Quem administra os valores usa o painel (ou a API) com a **conta Murkio**.

## Quem é quem

| Papel | Quem é | O que pode |
|---|---|---|
| **Platform admin** | Role `admin` da conta Murkio na aplicação do Vault no Identity | Ver e editar **todas** as aplicações do Vault |
| **Admin** (da aplicação) | Quem criou a aplicação ou foi promovido | Convidar e remover membros, acessar e editar a tabela de segredos de todos, gerar e revogar tokens de qualquer um, ver toda a auditoria |
| **Developer** | Membro convidado | Editar só a própria tabela de segredos, gerar e revogar os próprios tokens, ver a própria auditoria |

O papel dentro de uma aplicação fica no **banco do Vault**, não no Identity. O JWT só diz quem é a pessoa.

## Fluxo

1. O admin cria a **aplicação** e vira `admin` dela.
2. O admin **convida** pessoas por e-mail. O convite é um link de uso único que vale 7 dias.
3. Cada membro tem a **própria tabela de segredos**. O admin acessa e edita a de todos.
4. O membro cria um **token** e recebe a **URI**. O token aparece uma única vez, e só o hash fica guardado.
5. O SDK usa `URI + token` e lê os valores. O admin pode revogar qualquer token, e vale na hora.
6. Tudo é auditado, inclusive leituras e acessos negados.

## Como é feito

- Um binário Go com SQLite ao lado, só `net/http` e as dependências aprovadas em [`dependencies.md`](../../.agents/rules/dependencies.md).
- Autenticação de pessoas pelo JWT do Identity (Ed25519, validado localmente pelo JWKS) e de máquinas pelo token `sk_vt_`.
- Atrás do Caddy em `api.murkio.com/v1/vault`. Rate limit, TLS e log de acesso são do Caddy, não do Vault.
- gRPC privado em `127.0.0.1` para outros serviços da plataforma.

Próximo: [Conceitos](concepts.md).
