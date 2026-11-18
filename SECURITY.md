# Política de segurança

O Vault guarda segredos de outras pessoas, então a segurança dele é o produto. O código é público para que qualquer pessoa possa conferir o que ele faz com os dados.

## Como reportar uma vulnerabilidade

**Não abra uma issue pública** para uma falha de segurança. Use um destes canais:

1. **GitHub (preferido):** aba **Security** do repositório, botão **Report a vulnerability** (relato privado, o GitHub Security Advisories).
2. **E-mail:** `team@murkio.com`.

Inclua, se puder:

- o que acontece e qual o impacto (vazamento de valor, bypass de permissão, escalada de papel, negação de serviço...);
- a versão ou o commit, e como o Vault foi configurado;
- os passos para reproduzir (requisições, `.http` ou teste);
- se a falha já está sendo explorada ou se outras pessoas sabem dela.

**Nunca envie valores reais de segredo, tokens (`sk_vt_`, `inv_`), JWTs ou chaves (`ENCRYPTION_KEY`) em um relato.** Use dados fictícios.

### O que você pode esperar

- Confirmação de recebimento em até **5 dias úteis**.
- Uma avaliação inicial e, se for uma falha real, um plano de correção. Falhas que expõem valores de segredo ou quebram o isolamento entre aplicações têm prioridade máxima.
- Aviso quando a correção for publicada e, se você quiser, crédito pelo achado.
- Divulgação coordenada: pedimos que você não divulgue a falha antes da correção estar disponível.

Estes prazos são metas, não garantias contratuais.

## Versões com suporte

O Vault ainda não tem versões numeradas. Correções de segurança entram na branch `main`. Rode sempre o commit mais recente.

## O que está no escopo

- O código deste repositório (`cmd/`, `internal/`, `pkg/`, `migrations/`, `proto/`).
- Falhas de autenticação, autorização, isolamento entre aplicações, cifragem, auditoria, injeção, validação de entrada e vazamento de valor em resposta, log ou erro.

Fora do escopo:

- configuração do **seu** deploy (porta HTTP aberta fora do Caddy, chave guardada junto do banco, `.env` com permissão aberta);
- o Caddy, o Cloudflare, o Identity e o Messaging, que têm os próprios repositórios;
- ataques que exigem acesso ao servidor ou à `ENCRYPTION_KEY` (quem tem o banco **e** a chave lê tudo, por desenho);
- limite de taxa, que é da borda ([Deploy](docs/operations/deployment.md));
- os comportamentos já documentados em [Limitações atuais](docs/reference/limitations.md).

## Boa-fé

Pesquisa feita de boa-fé, dentro deste escopo, sem acessar dados de outras pessoas, sem degradar o serviço de terceiros e seguindo esta política, não será tratada como ataque. Teste em uma instância sua, nunca na de outra pessoa.

## Como o Vault protege os dados

Resumo das garantias. Detalhes em [Segurança](docs/security/overview.md).

- Valores cifrados em repouso com **AES-256-GCM**, com a chave **fora do banco**.
- O valor **nunca** aparece em listagem, erro, log ou auditoria. Só `reveal` e `runtime/env` devolvem valor, e os dois são auditados.
- Tokens (`sk_vt_`) e convites (`inv_`) existem **só como hash** (SHA-256) e aparecem uma única vez.
- Auditoria **append-only** (o próprio banco recusa alterar) e **fail-closed**: sem auditar, a operação não acontece.
- Membro removido e token revogado perdem o acesso **na hora**.
- Isolamento por aplicação com parâmetros explícitos; uuid zero é recusado.

## Confira você mesmo

Você não precisa confiar na nossa palavra:

```sh
# 1. a única chamada de rede de saída do código de produção é a busca do JWKS no Identity (AUTH_URL)
grep -rn "http.NewRequest\|http.Client\|net.Dial\|smtp\.\|exec.Command" --include='*.go' . \
  | grep -v '_test.go' | grep -v 'pkg/rpc/' | grep -v 'pkg/mocks/'

# 2. as dependências de terceiros (modernc.org/sqlite, yaml, grpc, protobuf e testify nos testes)
cat go.mod

# 3. nenhum valor de segredo vai para log ou auditoria: veja o que cada ação grava
grep -rn "Metadata" --include='*.go' internal/restful/*/useCases | grep -v _test.go

# 4. os testes que provam o isolamento e a cifragem
go test -race ./...
```

O Vault não envia telemetria nem contata nenhum serviço da Murkio por conta própria: o único tráfego de saída é a leitura do JWKS do `AUTH_URL` que **você** configura.

Para publicar uma instância sua com segurança, siga [Deploy](docs/operations/deployment.md) e o checklist.
