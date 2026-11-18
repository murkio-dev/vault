# Rotacionar a `ENCRYPTION_KEY`

```sh
NOVA=$(openssl rand -base64 32)
```

1. Antes: `ENCRYPTION_KEY=<A>`.
2. Troque para:

   ```sh
   ENCRYPTION_KEY=<NOVA>
   ENCRYPTION_KEYS_PREVIOUS=<A>
   ```

3. Reinicie o Vault. Ele abre valores com `A` e já cifra os novos com `NOVA`.
4. Deixe os segredos serem lidos. Cada valor aberto com `A` é recifrado com `NOVA`.
5. Quando todos tiverem sido lidos, tire `A` de `ENCRYPTION_KEYS_PREVIOUS` e reinicie.

Não remova `A` antes: um valor que ninguém leu ainda está cifrado com ela.

Detalhes e cuidados: [Rotação da `ENCRYPTION_KEY`](../operations/key-rotation.md).
