# Cifragem

Os valores dos segredos ficam no SQLite **cifrados**. A chave fica fora do banco, em `ENCRYPTION_KEY`.

## Formato

- Algoritmo: **AES-256-GCM** (`pkg/secretbox`).
- Chave: 32 bytes em base64 padrão (`openssl rand -base64 32`).
- O valor guardado é `nonce (12 bytes) || ciphertext+tag`, com nonce aleatório por cifragem.
- Dados autenticados adicionais (AAD): `secret_version:<id da linha em secret_versions>`. Um ciphertext copiado para outra linha **não abre**.

Quem fala com o `secretbox` é só `internal/adapters/secretvalues`. Use cases usam o contrato `ISecretValues`.

## Chaveiro

`ENCRYPTION_KEY` é a chave **ativa** e `ENCRYPTION_KEYS_PREVIOUS` é uma lista, separada por vírgula, das antigas. Ao abrir um valor o Vault tenta a ativa e depois as antigas, na ordem. Uma chave anterior igual à ativa é recusada no boot.

Quando um valor só abre com uma chave antiga, o Vault o **recifra com a ativa** e grava de volta. Depois que tudo foi lido (ou recifrado), a chave antiga pode sair. Passo a passo: [Rotação da `ENCRYPTION_KEY`](../operations/key-rotation.md).

## Versões

Cada alteração cria uma linha nova em `secret_versions`. Triggers do banco proíbem apagar uma versão e proíbem alterar qualquer coisa além do `ciphertext` (que só muda na recifragem).

## O que acontece quando falha

- Chave inválida no boot: o Vault não sobe e a mensagem de erro nunca contém a chave.
- Valor que não abre com nenhuma chave: a chamada vira `500` com mensagem fixa, sem expor a causa.
- `reveal` decifra e audita na mesma transação: se a auditoria falhar, o valor não sai.

## Guarde a chave

Quem tem o banco **e** a chave lê tudo. Perder a chave é perder os valores. Faça o backup do `vault.db` e da `ENCRYPTION_KEY` em lugares separados.
