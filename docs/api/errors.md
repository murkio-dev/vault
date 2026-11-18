# Erros

Todo erro segue a estrutura da [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) (problem details), entregue no formato negociado pelo `Accept` (veja [Formatos](formats.md)).

```json
{
  "error": {
    "type": "https://murkio.com/en-us/docs/vault/errors/#validation",
    "title": "Validation error",
    "status": 400,
    "detail": "The field name is invalid.",
    "instance": "/api/v1/applications",
    "invalid_params": [
      {"field": "name", "reason": "<motivo>", "value": "Meu App"}
    ]
  }
}
```

| Campo | Significado |
|---|---|
| `type` | URI do tipo do erro (`…/errors/#<tipo>`) |
| `title` | nome curto do tipo |
| `status` | status HTTP |
| `detail` | explicação em inglês, sem valor de segredo e sem mensagem interna |
| `instance` | o path da requisição |
| `invalid_params` | os campos inválidos: `field`, `reason` e, quando seguro, `value` |

## Tipos

| `type` | Status | Exemplos |
|---|---|---|
| `#validation` | `400` | nome fora do padrão, `limit` inválido, corpo com campo desconhecido, mais de um item numa rota de um recurso só |
| `#unauthorized` | `401` | credencial ausente, inválida, do tipo errado, revogada ou expirada |
| `#forbidden` | `403` | membro sem permissão para a ação |
| `#not-found` | `404` | não existe, ou o chamador não pode saber que existe |
| `#conflict` | `409` | aplicação ou segredo já existe, último admin, já é membro, tabela cheia (100 segredos), segredo referenciado, membro suspenso criando token |
| `#internal` | `500` | erro de banco ou de cifragem |
| `#unavailable` | `503` | dependência indisponível |

## Regras que valem para todos

- **`404` não revela o que existe.** Quem não é membro (ou foi suspenso ou removido) recebe o mesmo `404` de um id que não existe.
- **`500` não vaza a causa.** O `detail` é fixo (`An unexpected error occurred. Try again later.`) e a mensagem interna não sai na resposta.
- **Valor de segredo nunca aparece** em erro, `invalid_params` ou log. Em campos que não são segredo (o nome de uma aplicação, por exemplo) o `value` pode voltar para ajudar a corrigir.
- Em lote, cada item tem o seu erro, na ordem do lote. Veja [Lote](batch.md).

## No gRPC

Os erros viram códigos gRPC: `400` e `422` em `InvalidArgument`, `401` em `Unauthenticated`, `403` em `PermissionDenied`, `404` em `NotFound`, `409` em `AlreadyExists`, `429` em `ResourceExhausted`. O resto vira `Internal` com o texto fixo `internal error`. Veja [gRPC interno](grpc.md).
