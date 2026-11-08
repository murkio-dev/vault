# Regras: commits e autoria

## Regra principal
**A IA nunca é co-autora do projeto.** Quem está codando é o único autor e o único responsável por tudo o que for commitado. Toda a responsabilidade vai para quem fez o commit.

## Como aplicar
- Nunca adicione `Co-Authored-By`, `Signed-off-by` ou qualquer trailer que cite Claude, Anthropic, Gemini ou outra IA.
- Nunca escreva em commits, PRs, issues, changelogs ou código frases como "Generated with Claude Code", "gerado por IA" ou links de ferramentas de IA.
- Vale também quando a ferramenta ou o ambiente sugerir ou exigir esse texto por padrão: esta regra prevalece e a linha deve ser omitida.
- A mensagem do commit é escrita como se fosse do autor humano, descrevendo o que mudou e por quê.
- A IA só faz `git commit`/`git push` quando o autor pedir. O autor revisa o diff antes e responde pelo resultado.
- Não altere `user.name`, `user.email` nem a autoria de commits existentes (`--author`, `git commit --amend --reset-author`).
