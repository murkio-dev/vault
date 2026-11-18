# Publicar atrás do Caddy

O Vault não faz TLS, rate limit nem log de acesso: isso é do Caddy (`infra/`).

1. **Suba o Vault** com as variáveis no ambiente do processo e o `APPLICATION_BACKEND` com a URL pública (`https://api.murkio.com/v1/vault`).
2. **Aponte o Caddy** para ele em `infra/.env`:

   ```sh
   VAULT_APP_SITE=api.murkio.com
   VAULT_APP_UPSTREAM=<host do vault>:8083
   ```

   Sem `http://` no host, o Caddy emite e renova o TLS sozinho.
3. `make up` em `infra/` (constrói e sobe o Caddy).
4. **Feche a porta HTTP do Vault** no firewall, deixando só o Caddy alcançá-la. O gRPC já escuta só em `127.0.0.1`.
5. **Teste**:

   ```sh
   curl -i https://api.murkio.com/v1/vault/applications -H "Authorization: Bearer $ACCESS_TOKEN"
   curl -i https://api.murkio.com/qualquer-outra-coisa      # 404
   ```

O Caddy troca `/v1/vault` por `/api/v1`, limita o corpo a 64 KB e aplica o rate limit das rotas sensíveis. Detalhes e limites em [Deploy](../operations/deployment.md) e em `infra/README.md`.

Se o Auth usar o mesmo host (`api.murkio.com`), junte os dois blocos num só e roteie por `/v1/vault/*`, porque o Caddy não aceita dois sites com o mesmo host.
