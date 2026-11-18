# Limitações atuais

Comportamentos de hoje que costumam surpreender. Não são erros de documentação: é assim que o código funciona. Itens marcados com (planejado) estão no desenho do produto ([`schema/vault.md`](../../../schema/vault.md)) e ainda não existem.

## API

- **Lista de convites sem paginação.** `GET …/invitations` devolve os 200 mais novos, sem `page`. Listas de segredos, versões e tokens também não paginam ([Paginação](../api/pagination.md)).
- **Auditoria: `limit` acima de 200 volta para 100.** Não é cortado em 200, como nas outras listas.
- **Lote só no `POST` de segredos.** `PUT`, `PATCH` e `DELETE` de segredo existem só com `/{id}`.
- **Origens `generated` e `dynamic` (planejado).** Aceitam-se só `static` e `reference`; as outras dão `400`. O mesmo vale para banco dinâmico, rotação automática, alerta de impressão e a restrição de 1 URI por máquina.
- **O e-mail do convite não é conferido.** Quem tiver o token e uma conta Murkio entra, seja qual for o e-mail da conta. O token é a prova.
- **Convites repetidos.** Não há checagem de convite pendente para o mesmo e-mail nem de e-mail que já é membro (o `409` vem na hora de aceitar).
- **`member.added` nunca é gravada.** A ação existe no código, mas entrar por convite grava `invitation.accepted`, e criar a aplicação grava `application.created`.

## gRPC

- O `ApplicationService` não tem `Upgrade` (o PATCH).
- Os comentários do `.proto` e o `Caddyfile` ainda citam o prefixo `vt_`. O real é `sk_vt_`.

## Concorrência e consistência

- **O limite de 100 segredos por membro** é conferido antes da transação. Duas criações simultâneas podem passar do limite.
- **Aceitar dois convites da mesma aplicação ao mesmo tempo** com a mesma conta pode terminar em `500` em vez de `409` (a violação de unicidade do membro não é traduzida).
- **Revogar token:** um erro de banco ao buscar o token vira `404`.
- **Referências** são resolvidas sem limite de profundidade. A regra de "referência não aponta para referência" é aplicada na criação, não por `CHECK` no banco. Uma linha editada à mão pode criar um laço.
- A permissão do dono do alvo de uma referência é conferida **na criação**, não a cada leitura.

## Operação

- **Borda.** O HTTP escuta em todas as interfaces e o IP da auditoria vem de `CF-Connecting-IP`, `X-Forwarded-For` ou da conexão. Só é confiável atrás do Caddy ([Deploy](../operations/deployment.md)).
- **Retenção.** Nenhum job chama o expurgo da auditoria (`PurgeExpired`) nem publica a outbox ([Retenção](../operations/retention.md)).
- **Sem health check.** Teste a porta.
- **Falha no desligamento.** Se o `graceful.Run` devolver erro, o erro é logado e o processo sai com código `0`.
- **A chave de cifragem não tem comando de recifragem em massa** ([Rotação](../operations/key-rotation.md)).
- **`SQLITE_TEST_PATH`** é lido pela configuração de produção (só serve em teste).
- **Make.** O `makefile` do vault só tem `make proto`. Os alvos `api`, `build`, `sudo-delete-*` e `gen-data` ainda não existem, e `cmd/cli`, `cmd/mcp` e `cmd/jobs` também não.
- **`go.mod`.** `grpc` e `protobuf` aparecem como `// indirect`, embora o código os importe direto. Um `go mod tidy` ajusta.
