# Regras: API REST

## Rotas
Prefixo `/api/v1/<service>` (ex: `/api/v1/applications`).

### Batch ou entidade única (regra geral)
Sempre que fizer sentido, a rota aceita **lote ou entidade única**, e o use case é escrito para os dois casos. Reaplique essa regra em todo use case novo.

- **Body**: pode ser um objeto (`{...}`) ou um array (`[{...}, {...}]`). O formato vem do body, não da rota. `httpx.BindBody` normaliza ambos para `[]T`, e o use case sempre itera sobre uma lista.
- **Path**: `{id}` é opcional nas rotas de escrita.
  - Com `{id}`: opera em exatamente um item, e o id do path sobrescreve o `id` do body. Body com mais de um item é erro de validação.
  - Sem `{id}`: o body traz os ids (um objeto ou um array). Em `DELETE`, o body pode ser uma lista de ids (`["<uuid>", ...]`) ou objetos `{"id": "<uuid>"}`.
- Cada rota é registrada com e sem `{id}` e com e sem barra final (`/{$}`).
- **Leitura**: `GET /<service>` lista (sempre array, via `RespondList`). `GET /<service>/{id}` devolve uma entidade.
- **Paginação**: o padrão é `page`/`limit` (`limit` padrão e máximo de 150, objeto `page` na resposta), usado em aplicações e membros. Exceções que existem hoje: a auditoria pagina por **cursor** (`limit` padrão 100 e máximo 200, `page.next_cursor`); convites devolvem uma lista fixa dos 200 mais novos; segredos, versões e tokens devolvem a lista inteira (tetos de 100, 500 e 200). Lista nova pagina por `page`/`limit`, a menos que o volume seja limitado por regra de negócio.

### Regras do use case para batch
- Cada item é processado de forma independente. A falha de um item não aborta os outros.
- Ids repetidos no mesmo request são calculados no controller (`DuplicatedIDs`). O use case reporta o erro uma vez por id e não processa nenhuma das ocorrências.
- Id vazio (uuid zero) gera erro de validação para o item.
- `Data` recebe os itens que deram certo e `Errors` os que falharam.

### Rotas com segredo, papel ou credencial
Rota que recebe ou devolve valor de segredo, token, convite ou papel não usa a negociação de formato livre:

- **Corpo**: `httpx.BindJSON`/`httpx.BindOne`. Só JSON (o `Content-Type` não decide, então YAML e XML nunca são interpretados) e **campo desconhecido é erro** `400`. O cliente que manda `created_by`, `member_id` ou `role` onde não cabe não vê o campo ignorado em silêncio. Objeto ou lista de um item vale como um item; dois ou mais itens numa rota de um recurso só é erro.
- **Resposta**: `httpx.RespondJSON`, sempre JSON, ignorando o `Accept`. YAML, XML e a tabela de texto são outros renderizadores sobre o mesmo dado e não têm por que existir ali. Vale para `reveal`, `runtime/env`, criação de token e de convite.
- **Nunca** repetir o valor recebido em `invalid_params`, `detail` ou log.
- Toda a API sai com `Cache-Control: no-store`.
- Só aplicações (criar, atualizar, apagar) e a **criação** de segredos (até 50 itens) aceitam lote. `PUT`, `PATCH` e `DELETE` de segredo existem só com `{id}`, e as demais escritas operam num recurso só.

### Resposta (`httpx.Respond`)
- Só `Data`: status de sucesso da rota (`201` create, `200` update/upgrade/get, `204` delete).
- Só erros: status do primeiro erro.
- `Data` e `Errors` juntos: `207 Multi-Status`.
- Um único item vira objeto: `data: {...}` ou `error: {...}`. Dois ou mais itens ficam como lista (`data: [...]`, `errors: [...]`). Listagens nunca são desembrulhadas.
- Formato por `Accept`: JSON (padrão), XML, YAML ou `text/plain` (tabela). Erros seguem RFC 9457.
