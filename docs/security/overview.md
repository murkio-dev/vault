# Segurança: visão geral

Regras que valem para todo o código do Vault e que os testes provam.

1. **O valor nunca fica na entidade serializável** nem em listagem. Só `reveal` e `runtime/env` devolvem valor, por um tipo de saída próprio.
2. **Rotas que devolvem valor respondem só JSON** e com `Cache-Control: no-store`. Toda a API sai com `no-store`.
3. **Erros nunca repetem valores.** Acesso negado e recurso inexistente respondem igual (`404`), então ninguém descobre o que existe.
4. **Log e auditoria guardam ids e nomes, nunca valores** nem corpo de requisição.
5. **Valores cifrados em repouso** com AES-256-GCM, chave fora do banco e rotação por chave anterior. **Token e convite só existem como hash** (SHA-256).
6. **Auditoria append-only e fail-closed**: o próprio banco recusa alterar ou apagar, e se não der para auditar a operação não acontece. Leituras e acessos negados também entram.
7. **Membro removido e token revogado perdem acesso na hora.** O vínculo e o token são conferidos no banco a cada requisição, mesmo com o JWT ainda dentro do prazo.

## Isolamento por aplicação

A aplicação (e o dono, quando o dado é do membro) entram como **parâmetros explícitos** dos métodos dos repositories. Nada de tenant no `ctx`. Parâmetro zerado (uuid zero) é recusado com erro, sem tocar o banco, e nunca vira "sem filtro". Todo repository de tenant tem teste provando que outra aplicação não enxerga nem altera o dado.

## Credenciais

| Credencial | Defesa |
|---|---|
| JWT | Ed25519 validado localmente (JWKS), `iss`/`aud`/`aid` conferidos, 30 s de folga |
| Token `sk_vt_` | 32 bytes aleatórios, só o SHA-256 guardado, aparece uma única vez, revogável na hora |
| Convite `inv_` | 32 bytes aleatórios, só o hash guardado, uso único, 7 dias |

Credencial do tipo errado é recusada sem consulta. Toda falha de autenticação devolve o mesmo corpo.

## O que o Vault **não** faz

O Vault não implementa rate limit, ban progressivo, log de acesso, TLS nem resolução confiável do IP do cliente. Isso é da borda (Caddy + Cloudflare). O Vault só recebe requisições já filtradas. Veja [Deploy](../operations/deployment.md).

O IP gravado na auditoria vem de `CF-Connecting-IP`, `X-Forwarded-For` ou da conexão. Só é confiável com o Vault atrás do Caddy. Não exponha a porta HTTP diretamente.

## Páginas relacionadas

- [Política de segurança e como reportar uma vulnerabilidade](../../SECURITY.md)
- [Cifragem](encryption.md)
- [Permissões](permissions.md)
- [Rotação da chave](../operations/key-rotation.md)
- [Limitações atuais](../reference/limitations.md)
