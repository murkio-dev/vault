# Vault: contrato da API

Contrato HTTP do `vault/`. O produto e as decisões estão em `../../schema/vault.md`. Nada aqui está em produção, então este é o formato definitivo: mudar depois é quebra de contrato.

## Convenções

- Prefixo `/api/v1`. O Caddy expõe como `api.murkio.com/v1/vault` (sem subdomínio) e só abre depois da autenticação.
- Corpo e resposta em **snake_case**. Ids são UUIDv7. Datas em RFC 3339 UTC.
- Respostas de sucesso: `{"data": ...}` (item único vira objeto, listas são sempre arrays; listas levam `"page"`).
- Erros: `{"error": {...}}` no formato RFC 9457 (`type`, `title`, `status`, `detail`, `instance`, `invalid_params`).
- Só a criação de **aplicações** e de **segredos** aceita um lote (`[...]`, até 50 segredos); em lote cada item é independente e a resposta é `207` se houver sucesso e erro misturados. As demais escritas recebem um objeto.
- Os corpos de segredos, convites, membros e tokens são **JSON estrito**: YAML e XML são recusados, e um campo desconhecido (`created_by`, `member_id`, `role` onde não cabe...) é `400`, nunca ignorado em silêncio.
- Status: `201` cria, `200` lê/atualiza, `204` apaga/revoga, `207` lote parcial, `400` validação, `401` sem credencial válida, `403` sem permissão, `404` não existe **ou não é visível para você**, `409` conflito, `429` limite (do Caddy).
- `404` e `403`: quem **não é membro** da aplicação recebe `404` em tudo (não se revela o que existe). Quem é membro e não tem a permissão recebe `403`.
- Listas comuns: `?page=1&limit=50` (máximo 150), com `"page": {"page","limit","total","has_next","has_prev"}`. A **auditoria** usa cursor: `?cursor=<opaco>&limit=100`, com `page.next_cursor`.
- Rotas que devolvem valor de segredo respondem **só JSON**, com `Cache-Control: no-store`. As demais seguem a negociação por `Accept`.

## Autenticação

| Quem | Header | Uso |
| --- | --- | --- |
| Pessoa (admin, dev) | `Authorization: Bearer <JWT do Auth>` | Gestão: tudo menos `/runtime` |
| Máquina (SDK) | `Authorization: Bearer vt_<token>` | Só `GET /runtime/{uri_id}/env` |

- O JWT é validado localmente pelo JWKS do Auth (Ed25519, `iss`, `aud` = a aplicação do Vault no Auth, `exp`). Falha ou ausência: `401` genérico, sem dizer o motivo.
- O token `vt_` é procurado pelo hash. Revogado, expirado ou de membro removido/suspenso: `401`.
- Em toda chamada o Vault confere no banco se o membro continua ativo; remover alguém vale na hora, mesmo com o JWT ainda no prazo.
- O papel (`admin`/`developer`) é da aplicação do Vault, guardado no banco do Vault. A role `admin` do Auth na aplicação do Vault vira **platform admin**.

## Permissões

`—` = negado. "próprio" = só a tabela, tokens e auditoria do próprio membro.

| Ação | Platform admin | Admin | Developer |
| --- | --- | --- | --- |
| Criar aplicação | sim | sim (vira admin dela) | sim (vira admin dela) |
| Ver aplicação | todas | as dele | as dele |
| Editar/apagar aplicação | sim | sim | — |
| Listar/alterar/remover membros | sim | sim | — |
| Convidar, listar, revogar convites | sim | sim | — |
| Listar/ver segredos (metadados) | todos | de todos | próprios |
| Criar/editar/apagar segredos | todos | de todos | próprios |
| Revelar valor | todos | de todos | próprios |
| Criar token | de qualquer membro | de qualquer membro | próprios |
| Listar tokens | todos | de todos | próprios |
| Revogar token | todos | de todos | próprios |
| Ler auditoria | toda | toda | própria |

A decisão é uma função única e pura (`internal/entities/policy.go`), testada em matriz. Regra de proteção: não é possível remover, suspender ou rebaixar o **último admin** de uma aplicação (`409`). A contagem e a mudança estão na mesma transação, então dois pedidos simultâneos não conseguem tirar os dois últimos.

## Aplicações

| Método | Rota | Corpo | Resposta |
| --- | --- | --- | --- |
| `POST` | `/applications` | `{"name"}` | `201` aplicação |
| `GET` | `/applications` | — | `200` lista (as minhas; platform admin vê todas) |
| `GET` | `/applications/{app}` | — | `200` |
| `PUT` | `/applications/{app}` | `{"name"}` | `200` |
| `PATCH` | `/applications/{app}` | `{"name"?}` | `200` (campo omitido não muda) |
| `DELETE` | `/applications/{app}` | — | `204` (soft delete; apaga segredos e revoga tokens) |

```json
{"data": {"id": "…", "name": "minha-app", "created_by": "<user_id>", "created_at": "…", "updated_at": null}}
```

`name`: `^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$` (3 a 50 caracteres), único. O `created_by` **não vem do corpo**: é o `sub` do JWT.

## Membros

| Método | Rota | Corpo | Resposta |
| --- | --- | --- | --- |
| `GET` | `/applications/{app}/members` | — | `200` lista |
| `GET` | `/applications/{app}/members/me` | — | `200` o próprio membro |
| `PATCH` | `/applications/{app}/members/{member}` | `{"role"?, "status"?}` | `200` |
| `DELETE` | `/applications/{app}/members/{member}` | — | `204` (revoga os tokens dele e apaga os segredos dele; as versões cifradas e a auditoria ficam) |

```json
{"data": {"id": "…", "application_id": "…", "user_id": "…", "uri_id": "…", "role": "developer", "status": "active", "created_at": "…"}}
```

`uri_id` identifica a tabela de segredos do membro (a URI do SDK). É estável: gerar ou revogar tokens não o muda. Sozinho não dá acesso.

`role`: `admin` | `developer`. `status`: `active` | `suspended`.

## Convites

| Método | Rota | Corpo | Resposta |
| --- | --- | --- | --- |
| `POST` | `/applications/{app}/invitations` | `{"email", "role"}` | `201` convite + `token` (**uma vez**) |
| `GET` | `/applications/{app}/invitations` | — | `200` lista (sem token), com `status` derivado: `pending`, `accepted`, `revoked` ou `expired` |
| `DELETE` | `/applications/{app}/invitations/{id}` | — | `204` revoga |
| `POST` | `/invitations/accept` | `{"token"}` | `201` membro |

- O convite vale 7 dias e é de uso único. Só o hash do token fica guardado.
- O e-mail vai para a outbox como um `email.send` (template `vault_invitation`, exchange `messaging.commands`), e o Messaging o envia quando os jobs existirem; o token no payload é apagado depois de publicado. Enquanto isso, o `token` da resposta de criação é o único jeito de entregar o convite.
- Quem já é membro e tenta aceitar outro convite da mesma aplicação recebe `409`, e o convite não é consumido. Dois aceites simultâneos do mesmo link terminam com um só membro.
- `accept` exige o JWT de **quem aceita**; o convite vira membro ligado ao `sub` dessa conta. Token inválido, expirado, usado ou revogado responde igual: `404`.
- O JWT do Auth não carrega e-mail, então quem tem o link entra. Casar com o e-mail da conta depende do gRPC auth↔vault.

## Segredos

A tabela pertence a um membro. `{member}` pode ser `me`.

| Método | Rota | Corpo | Resposta |
| --- | --- | --- | --- |
| `GET` | `/applications/{app}/members/{member}/secrets` | — | `200` lista de **metadados** |
| `POST` | `…/secrets` | `{"name","type","value"}` ou `{"name","type","source":"reference","reference_secret_id"}` | `201` metadados |
| `GET` | `…/secrets/{id}` | — | `200` metadados |
| `PUT` | `…/secrets/{id}` | `{"value"}` | `200` metadados (cria **nova versão**) |
| `PATCH` | `…/secrets/{id}` | `{"name"?}` | `200` metadados |
| `DELETE` | `…/secrets/{id}` | — | `204` (soft delete; versões ficam cifradas para auditoria) |
| `GET` | `…/secrets/{id}/versions` | — | `200` lista de versões (`version`, `created_by`, `created_at`; **sem valor**) |
| `POST` | `…/secrets/{id}/reveal` | `{"version"?}` | `200` com o valor |

```json
{"data": {"id": "…", "member_id": "…", "name": "DATABASE_URL", "type": "database_uri", "source": "static",
          "reference_secret_id": null, "version": 3, "created_by": "…", "created_at": "…", "updated_at": "…"}}
```

```json
{"data": {"id": "…", "name": "DATABASE_URL", "version": 3, "value": "postgres://…"}}
```

- **Metadados nunca trazem valor.** Só o `reveal` e o `/runtime/env` devolvem valor; os dois são auditados.
- `name`: `^[A-Z][A-Z0-9_]{0,127}$`, único por membro entre os ativos.
- `type` (v1): `string`, `password`, `url`, `database_uri`, `api_key`, `jwt_key`, `paseto_key`. Cada tipo valida o formato do valor. Valor: até 32 KiB.
- `source` (v1 aceita `static` e `reference`; `generated` e `dynamic` ficam reservados e respondem `400`). Uma `reference` aponta para um segredo **estático** da mesma aplicação e do **mesmo tipo**, e quem a cria precisa poder revelar o alvo (um developer só aponta para os próprios). Referência para referência é recusada, o que impede ciclos. Só um segredo estático tem valor próprio: `PUT` numa referência é `400`.
- Apagar um segredo que outros apontam é `409`. O reveal de uma referência devolve o valor atual do alvo.
- O `reveal` aceita `{"version": N}` (corpo opcional); versão que não existe é `404`.
- Erros de validação **não repetem o valor** em `invalid_params`.

## Tokens (URI + token)

| Método | Rota | Corpo | Resposta |
| --- | --- | --- | --- |
| `POST` | `/applications/{app}/members/{member}/tokens` | `{"name","expires_at"?}` | `201` com `token` (**uma vez**) e `uri` |
| `GET` | `…/tokens` | — | `200` lista (sem token) |
| `POST` | `…/tokens/{id}/revoke` | — | `204` |

```json
{"data": {"id": "…", "name": "notebook", "prefix": "vt_a1b2c3", "uri_id": "…", "uri": "<base pública>/<uri_id>",
          "token": "vt_…", "expires_at": null, "created_at": "…"}}
```

A `uri` é a base pública do Vault (configuração do deploy) mais o `uri_id` do membro. O `last_used_at` é atualizado no máximo uma vez por minuto. Depois da criação só aparecem `id`, `name`, `prefix`, `uri_id`, `last_used_at`, `expires_at` e `revoked_at`.

## Runtime (SDK)

`GET /runtime/{uri_id}/env` com `Authorization: Bearer vt_…`. Devolve as variáveis do membro dono do token, já com as `reference` resolvidas. O `uri_id` do caminho precisa ser o do membro dono do token; senão, `404`.

```json
{"data": {"uri_id": "…", "variables": [{"name": "DATABASE_URL", "type": "database_uri", "version": 3, "value": "postgres://…"}]}}
```

- `?names=A,B` limita às chaves pedidas. Chave inexistente é omitida (não é erro).
- `Cache-Control: no-store`. O Caddy limita a taxa por token.
- Cada leitura gera `env.read` na auditoria, na mesma transação (nunca valores). Sem conseguir gravar a trilha, o ambiente não sai (`500`).
- O token errado, revogado, vencido, de membro suspenso/removido ou de aplicação apagada responde sempre o mesmo `401`, sem motivo.

## Auditoria

`GET /applications/{app}/audit-logs?cursor=&limit=&action=&actor_id=&target_id=&outcome=&from=&to=`

```json
{"data": [{"id": "…", "action": "secret.revealed", "outcome": "allowed", "actor_type": "user", "actor_id": "…",
           "token_id": null, "target_type": "secret", "target_id": "…", "ip": "…", "user_agent": "…",
           "request_id": "…", "metadata": {"name": "DATABASE_URL", "version": 3}, "created_at": "…"}],
 "page": {"next_cursor": "…"}}
```

- Só leitura: não existe rota de escrita, e o banco **recusa** `UPDATE` e `DELETE` na tabela.
- `metadata` guarda nomes, versões e ids. **Nunca valores.**
- Fail-closed: se a linha de auditoria não puder ser gravada, a operação não acontece.
- Ações: `application.created|updated|deleted`, `member.updated|removed`, `invitation.created|accepted|revoked`, `secret.created|updated|renamed|deleted|revealed`, `token.created|revoked`, `env.read`, `access.denied`. Leituras de metadados e da própria trilha não são auditadas.
- O developer só enxerga as linhas em que ele é o autor (o filtro `actor_id` é trocado por ele). O admin e o platform admin veem tudo da aplicação.
- O `cursor` é o `next_cursor` da página anterior; `limit` vai de 1 a 200 (padrão 100, acima do teto é aparado). Filtros: `action`, `actor_id`, `target_id`, `outcome` (`allowed`|`denied`), `from`, `to` (RFC 3339).

## Erros (`type`)

Base `https://murkio.com/en-us/docs/vault/errors/`:

| Fragmento | Status | Quando |
| --- | --- | --- |
| `#validation` | 400 | Corpo, parâmetro ou id inválido |
| `#unauthorized` | 401 | Credencial ausente, inválida, expirada ou revogada |
| `#forbidden` | 403 | Membro sem permissão para a ação |
| `#not-found` | 404 | Não existe, ou não é visível para você |
| `#conflict` | 409 | Nome repetido, último admin, referência em ciclo |
| `#internal` | 500 | Falha interna (sem detalhe) |
| `#unavailable` | 503 | Banco fora (só no `ready`) |
