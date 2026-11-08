# Regras: API REST

## Rotas
Prefixo `/api/v1/<service>` (ex: `/api/v1/applications`).

### Batch ou entidade única (regra geral)
Sempre que fizer sentido, a rota aceita **lote ou entidade única**, e o use case é escrito para os dois casos. Reaplique essa regra em todo use case novo.

- **Body**: pode ser um objeto (`{...}`) ou um array (`[{...}, {...}]`). O formato vem do body, não da rota. `utils.BindBody` normaliza ambos para `[]T`, e o use case sempre itera sobre uma lista.
- **Path**: `{id}` é opcional nas rotas de escrita.
  - Com `{id}`: opera em exatamente um item, e o id do path sobrescreve o `id` do body. Body com mais de um item é erro de validação.
  - Sem `{id}`: o body traz os ids (um objeto ou um array). Em `DELETE`, o body pode ser uma lista de ids (`["<uuid>", ...]`) ou objetos `{"id": "<uuid>"}`.
- Cada rota é registrada com e sem `{id}` e com e sem barra final (`/{$}`).
- **Leitura**: `GET /<service>` lista com paginação (sempre array, via `RespondList`). `GET /<service>/{id}` devolve uma entidade.

### Regras do use case para batch
- Cada item é processado de forma independente. A falha de um item não aborta os outros.
- Ids repetidos no mesmo request são calculados no controller (`DuplicatedIDs`). O use case reporta o erro uma vez por id e não processa nenhuma das ocorrências.
- Id vazio (uuid zero) gera erro de validação para o item.
- `Data` recebe os itens que deram certo e `Errors` os que falharam.

### Rotas com segredo, papel ou credencial
Rota que recebe ou devolve valor de segredo, token, convite ou papel não usa a negociação de formato livre:

- **Corpo**: `utils.BindJSON`/`utils.BindOne`. Só JSON (o `Content-Type` não decide, então YAML e XML nunca são interpretados) e **campo desconhecido é erro** `400`. O cliente que manda `created_by`, `member_id` ou `role` onde não cabe não vê o campo ignorado em silêncio. Objeto ou lista de um item vale como um item; dois ou mais itens numa rota de um recurso só é erro.
- **Resposta**: `utils.RespondJSON`, sempre JSON, ignorando o `Accept`. YAML, XML e a tabela de texto são outros renderizadores sobre o mesmo dado e não têm por que existir ali. Vale para `reveal`, `runtime/env`, criação de token e de convite.
- **Nunca** repetir o valor recebido em `invalid_params`, `detail` ou log.
- Toda a API sai com `Cache-Control: no-store`.
- Só aplicações e segredos aceitam lote; as demais escritas operam num recurso só.

### Resposta (`utils.Respond`)
- Só `Data`: status de sucesso da rota (`201` create, `200` update/upgrade/get, `204` delete).
- Só erros: status do primeiro erro.
- `Data` e `Errors` juntos: `207 Multi-Status`.
- Um único item vira objeto: `data: {...}` ou `error: {...}`. Dois ou mais itens ficam como lista (`data: [...]`, `errors: [...]`). Listagens nunca são desembrulhadas.
- Formato por `Accept`: JSON (padrão), XML, YAML ou `text/plain` (tabela). Erros seguem RFC 9457.
