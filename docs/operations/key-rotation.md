# Rotação da `ENCRYPTION_KEY`

Trocar a chave não exige parar o serviço nem reescrever o banco à mão: o Vault abre cada valor com a chave que servir e o **recifra com a ativa** na hora da leitura.

## Passo a passo

1. Gere a chave nova:

   ```sh
   openssl rand -base64 32
   ```

2. Mova a chave atual para a lista de anteriores e ponha a nova como ativa:

   ```sh
   ENCRYPTION_KEY=<chave nova>
   ENCRYPTION_KEYS_PREVIOUS=<chave antiga>
   ```

   Com várias chaves antigas, separe por vírgula. Uma anterior igual à ativa é recusada no boot.

3. Reinicie o Vault.

4. Deixe os valores serem lidos (`reveal` e `runtime/env`). Cada valor aberto com uma chave antiga é recifrado e gravado com a ativa.

5. Quando **todos** os valores foram recifrados, remova a chave antiga de `ENCRYPTION_KEYS_PREVIOUS` e reinicie.

## Cuidados

- Um valor que ninguém leu continua cifrado com a chave antiga. Se você tirar a chave antiga antes de ele ser lido, ele **não abre mais** (`500` na leitura). Enquanto houver dúvida, mantenha a chave antiga na lista.
- O Vault não tem comando que recifre tudo de uma vez. Para forçar, leia todos os segredos (por exemplo, o `runtime/env` de cada membro).
- Se a gravação da recifragem falhar, o erro é ignorado e a leitura segue com sucesso. O valor será recifrado na próxima leitura.
- Guarde a chave antiga até confirmar que nada depende dela.

## Se perder a chave

Sem a `ENCRYPTION_KEY` (e as anteriores que ainda cifram valores) os segredos não podem ser lidos. Não há recuperação.

Guia: [Rotacionar a `ENCRYPTION_KEY`](../guides/rotate-encryption-key.md). Detalhes do formato: [Cifragem](../security/encryption.md).
