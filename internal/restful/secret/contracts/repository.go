package secretcontracts

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
)

type ISecretRepository interface {
	// AddVersion grava uma NOVA versão do valor: a seguinte à
	// Secret.Version. Duas escritas na mesma versão: a segunda recebe sql.ErrNoRows.
	AddVersion(ctx context.Context, write entities.SecretWrite) (entities.Secret, error)
	Count(ctx context.Context, application, owner uuid.UUID) (int, error)
	CountReferences(ctx context.Context, application, secret uuid.UUID) (int, error)
	Create(ctx context.Context, write entities.SecretWrite) (entities.Secret, error)
	Delete(ctx context.Context, application, id uuid.UUID) error
	// FindById acha o segredo na aplicação, de qualquer dono (referências e
	// leitura do valor). Quem lê em nome de um membro usa FindByOwner.
	FindById(ctx context.Context, application, id uuid.UUID) (entities.Secret, error)
	// FindByOwner só acha o segredo da tabela do dono.
	FindByOwner(ctx context.Context, application, owner, id uuid.UUID) (entities.Secret, error)
	List(ctx context.Context, application, owner uuid.UUID, limit, offset int) ([]entities.Secret, int, error)
	// Rename renomeia o segredo; o valor e as versões não mudam.
	Rename(ctx context.Context, write entities.SecretWrite) (entities.Secret, error)
}

type IVersionRepository interface {
	FindByNumber(ctx context.Context, application, secret uuid.UUID, number int) (entities.SecretVersion, error)
	FindCurrent(ctx context.Context, application, secret uuid.UUID) (entities.SecretVersion, error)
	List(ctx context.Context, application, secret uuid.UUID, limit, offset int) ([]entities.SecretVersion, int, error)
	// Reseal troca o ciphertext de uma versão (a recifragem com a chave
	// ativa); o resto da versão não muda.
	Reseal(ctx context.Context, application, secret uuid.UUID, version entities.SecretVersion) (entities.SecretVersion, error)
}
