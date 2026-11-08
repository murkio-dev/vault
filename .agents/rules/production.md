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
A aplicação é empacotada como um binário único autossuficiente (12-Factor App). O uso de arquivos `.env` é estritamente para o ambiente de desenvolvimento. Em produção (FreeBSD), a aplicação não deve depender da leitura de arquivos locais de configuração e deve consumir as variáveis de ambiente diretamente do Sistema Operacional (`os.Getenv`).

## Borda HTTP (fora do vault)
O vault **não implementa middlewares** de borda. Rate limit, ban progressivo, logging de requisições, resolução de IP do cliente (`CF-Connecting-IP`, `X-Forwarded-For`), TLS e afins ficam em um serviço separado, na frente do vault, via Caddy, em outro repositório/pasta. Não crie `internal/middlewares/` nem lógica dessas responsabilidades no vault (a autenticação do JWT e o recovery de pânico não são borda: ficam em `pkg/authn` e `pkg/recovery`): ele só recebe requisições já filtradas e foca em regras de negócio e entrega REST. Isso também vale para logs de acesso, que são responsabilidade do Caddy. O vault só registra logs de aplicação (boot e erros de função).
