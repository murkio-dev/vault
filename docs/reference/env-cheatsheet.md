# Variáveis de ambiente

Tabela única. Explicação em [Configuração](../operations/configuration.md).

| Variável | Padrão | Obrigatória | Validação |
|---|---|---|---|
| `APPLICATION_PORT` | | sim | inteiro de 1 a 65535 |
| `APPLICATION_BACKEND` | vazio | não | sem validação (URL pública base) |
| `GRPC_PORT` | `50052` | não | inteiro de 1 a 65535, diferente de `APPLICATION_PORT` |
| `AUTH_URL` | | sim | `https`, ou `http` só em `localhost`/loopback; sem credenciais, query ou fragmento |
| `AUTH_APPLICATION_ID` | | sim | UUID não zero |
| `ENCRYPTION_KEY` | | sim | base64 de 32 bytes |
| `ENCRYPTION_KEYS_PREVIOUS` | vazio | não | base64 de 32 bytes, separadas por vírgula; nenhuma igual à ativa |
| `SQLITE_PATH` | `vault.db` | não | relativo à pasta do binário |
| `LOG_PATH` | `vault.log` | não | relativo à pasta do binário |
| `SQLITE_TEST_PATH` | vazio | não | só em testes |

Para as requisições `.http`: `hacks/http/.env` define `APPLICATION_BACKEND` (veja [Arquivos `.http`](../development/http-files.md)).
