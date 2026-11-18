package contracts

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
)

// ISecretValues cifra e decifra os valores. A chave nunca sai do processo.
// Seal cifra o valor de uma versão (versionID entra no AAD); Value devolve o
// valor de uma versão do segredo (0 = a atual), resolvendo a referência e
// recifrando com a chave ativa o que foi aberto com uma antiga.
type ISecretValues interface {
	Seal(versionID uuid.UUID, value string) ([]byte, error)
	Value(ctx context.Context, secret entities.Secret, version int) (value string, number int, err error)
}
