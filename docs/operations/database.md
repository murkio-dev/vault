# Banco (SQLite)

O Vault guarda tudo em **um arquivo SQLite** (`vault.db`, ao lado do binário), com o driver `modernc.org/sqlite` (Go puro, sem CGO).

## Como é aberto

- **Uma conexão só** (`SetMaxOpenConns(1)`): toda escrita é serializada.
- `journal_mode=WAL` e `synchronous=NORMAL` (para arquivos), `busy_timeout=10000`, `foreign_keys=ON`, `temp_store=memory`.
- Transações com `_txlock=immediate` e horários em UTC.
- Arquivos criados com permissão `0600` (a pasta, `0750`). Ao lado do `.db` aparecem `-wal` e `-shm`.
- `Transactor.Do` guarda a transação no `ctx`: um `Do` aninhado reaproveita a de fora, e erro ou pânico faz rollback.

## Migrations

Embutidas no binário (`migrations/embed.go`) e aplicadas **no boot**, em ordem de nome, uma transação por arquivo. O controle fica em `schema_migrations`. Os arquivos `.down.sql` existem para uso manual e o executor os ignora.

| # | Tabela | O que guarda |
|---|---|---|
| 0001 | `applications` | nome (3 a 50, único entre as ativas), `created_by`, `deleted_at` |
| 0002 | `members` | vínculo pessoa-aplicação, `role`, `status`, `uri_id` único |
| 0003 | `invitations` | e-mail, papel, `token_hash` único, `expires_at`, aceite e revogação |
| 0004 | `secrets` | metadados do segredo, `current_version`, referência |
| 0005 | `secret_versions` | `ciphertext` por versão (append-only) |
| 0006 | `access_tokens` | `token_hash` único, `prefix`, expiração, uso e revogação |
| 0007 | `audit_logs` | trilha append-only |
| 0008 | `outbox_events` | eventos para o Messaging (e-mail de convite) |

Todas usam `uuid` como texto e `deleted_at` para soft delete. Índices únicos parciais (`WHERE deleted_at IS NULL`) liberam nomes depois de apagar.

## Garantias no próprio banco

- `audit_logs`: trigger recusa **qualquer** `UPDATE`, e recusa `DELETE` de linhas com menos de um ano.
- `secret_versions`: trigger recusa `DELETE`, e recusa `UPDATE` de qualquer coluna além do `ciphertext`.
- `secrets`: `CHECK` garante que `source='reference'` se, e só se, há `reference_secret_id`. `type` e `source` são validados em Go, não por `CHECK`.
- `members`: único por `(application_id, user_id)` entre os vivos, e `uri_id` único.

## Isolamento por aplicação

Toda consulta de tenant recebe a aplicação (e o dono) como parâmetro e recusa o uuid zero. Os testes de repository usam dois tenants (`sqlitetest.Seed`) para provar que um não vê nem altera o outro.

## Backup

Pare a escrita ou use `sqlite3 vault.db ".backup vault.bak"`. Guarde o backup **separado** da `ENCRYPTION_KEY`. Sem a chave os valores não abrem.

## Recomeçar do zero (dev)

Apague `vault.db`, `vault.db-wal` e `vault.db-shm` com o serviço parado. O próximo boot recria tudo.
