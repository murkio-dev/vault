# Lote e entidade única

Onde faz sentido, a rota aceita **uma entidade ou uma lista**, e o formato vem do corpo, não da rota. O use case sempre itera sobre uma lista.

Aceitam lote:

| Rota | Limite |
|---|---|
| `POST /applications` | corpo de até 64 KiB |
| `PUT`, `PATCH`, `DELETE /applications[/{id}]` | corpo de até 64 KiB |
| `POST …/secrets` | até 50 itens |

As demais escritas (membros, convites, tokens e `PUT/PATCH/DELETE` de um segredo) operam em **um recurso só**, com corpo estrito (`400` se vier mais de um item).

```json
[
  {"name": "loja-api"},
  {"name": "x"},
  {"name": "loja-web"}
]
```

## Regras

- **Itens independentes.** A falha de um não aborta os outros.
- **`{id}` no path** opera em exatamente um item, e o id do path sobrescreve o `id` do corpo. Corpo com mais de um item é `400`.
- **Sem `{id}`**, o corpo traz os ids. Em `DELETE`, pode ser uma lista de ids (`["<uuid>", ...]`) ou objetos (`{"id": "<uuid>"}`).
- **Id repetido** no mesmo request é reportado **uma vez** por id, e nenhuma ocorrência é processada.
- **Id vazio** (uuid zero) é erro de validação do item.
- `data` recebe os itens que deram certo e `errors` os que falharam, na ordem do lote.

## Status

| Resultado | Status |
|---|---|
| tudo certo | `201` (criar), `200` (atualizar), `204` (apagar) |
| certo e errado juntos | `207 Multi-Status` |
| tudo errado | o status do primeiro erro |
| lista vazia | `400` ("empty payload") para aplicações |

```http
HTTP/1.1 207 Multi-Status

{
  "data": [{"id": "…", "name": "loja-api", ...}, {"id": "…", "name": "loja-web", ...}],
  "errors": [{"type": ".../errors/#validation", "status": 400, "invalid_params": [{"field": "name", ...}], ...}]
}
```

Um único item vira objeto (`data: {...}`), dois ou mais ficam como lista.
