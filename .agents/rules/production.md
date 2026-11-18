# Regras: ambiente e produção

Regras inegociáveis de ambiente, deploy e borda HTTP.

## Maquina operante em produção
- `OS`: FreeBSD
- `ram`: 4gb
- `disk`: 1tb
- `cpu`: 2 nucleos
- `docker/podman` false (vai rodar em um binario unico)

## Ambiente de desenvolvimento
- `OS`: linux(debian & wsl)/windows
- `ram`: unlimited
- `cpu`: unlimited
- `docker/podman`: true (em caso utilize containers tente primeiro com o podman, se falhar vai com o docker)

## .env e Deploy
A aplicação é empacotada como um binário único autossuficiente (12-Factor App). A configuração vem das variáveis de ambiente, e o `internal/config` as lê nesta ordem de prioridade: o **ambiente do processo**, o `.env` ao lado do binário e o `.env` do diretório atual. O arquivo nunca sobrescreve uma variável já definida e a ausência dele não é erro. Em produção (FreeBSD) defina as variáveis no ambiente do processo e não dependa do `.env`: ele é uma conveniência de desenvolvimento. `SQLITE_PATH` e `LOG_PATH` relativos valem a partir da pasta do binário.

## Borda HTTP (fora do vault)
O vault **não implementa middlewares** de borda. Rate limit, ban progressivo, logging de requisições, resolução de IP do cliente (`CF-Connecting-IP`, `X-Forwarded-For`), TLS e afins ficam em um serviço separado, na frente do vault, via Caddy, em outro repositório/pasta. Não crie `internal/middlewares/` nem lógica dessas responsabilidades no vault (a autenticação do JWT e o recovery de pânico não são borda: ficam em `pkg/authn` e `pkg/httpx`): ele só recebe requisições já filtradas e foca em regras de negócio e entrega REST. Isso também vale para logs de acesso, que são responsabilidade do Caddy. O vault só registra logs de aplicação (boot e erros de função).

O IP do cliente que vai para a auditoria (`entities/apiHeader.go`) vem de `CF-Connecting-IP`, `X-Forwarded-For` ou da conexão. Isso só é confiável com o vault atrás do Caddy: a porta HTTP do vault não deve ficar acessível fora dele. O servidor HTTP (`pkg/httpx`) traz limites de proteção do protocolo (timeouts, teto de cabeçalho e de corpo, recovery), que não são lógica de borda.
