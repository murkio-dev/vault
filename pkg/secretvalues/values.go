package secretvalues

import (
	"context"
	"database/sql"
	"errors"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/secretbox"
)

// Values é o contracts.ISecretValues: a ponte entre o ciphertext do banco e o
// valor em claro. Só ela fala com o secretbox.
type Values struct {
	Secrets  contracts.ISecretFinder
	Current  contracts.IVersionCurrentFinder
	Numbered contracts.IVersionNumberFinder
	Resealer contracts.IVersionResealer
	Box      *secretbox.Box
}

func NewValues(secrets contracts.ISecretFinder, current contracts.IVersionCurrentFinder, numbered contracts.IVersionNumberFinder, resealer contracts.IVersionResealer, box *secretbox.Box) Values {
	return Values{Secrets: secrets, Current: current, Numbered: numbered, Resealer: resealer, Box: box}
}

var _ contracts.ISecretValues = Values{}

// associatedData amarra o ciphertext à linha da versão: um valor copiado para
// outra versão (ou outro segredo) não abre.
func associatedData(versionID uuid.UUID) []byte {
	return []byte("secret_version:" + versionID.String())
}

func (v Values) Seal(versionID uuid.UUID, value string) ([]byte, error) {
	return v.Box.Seal([]byte(value), associatedData(versionID))
}

// Value devolve o valor da versão pedida (0 = a atual). Uma referência é
// resolvida para o valor ATUAL do segredo apontado, que precisa continuar
// ativo e na mesma aplicação (sql.ErrNoRows se não). Um valor aberto com chave
// antiga é recifrado com a ativa na hora, e a falha de recifrar não impede a
// leitura: o valor continua legível com a chave antiga. Os repositórios
// são chamados com a aplicação do segredo, nunca de outra.
//
// Entrada: o segredo já carregado (com a aplicação) e o número da versão.
// Saída: o valor em claro e o número da versão que o devolveu. Exemplo: uma
// referência na versão 0 devolve o valor atual do alvo, com o número do alvo.
func (v Values) Value(ctx context.Context, secret entities.Secret, version int) (string, int, error) {

	if secret.Source == entities.SourceReference {
		if secret.ReferenceSecretID == nil {
			return "", 0, sql.ErrNoRows
		}
		target, err := v.Secrets.FindById(ctx, secret.ApplicationID, *secret.ReferenceSecretID)
		if err != nil {
			return "", 0, err
		}
		return v.Value(ctx, target, 0)
	}

	var (
		stored entities.SecretVersion
		err    error
	)
	if version == 0 {
		stored, err = v.Current.FindCurrent(ctx, secret.ApplicationID, secret.ID)
	} else {
		stored, err = v.Numbered.FindByNumber(ctx, secret.ApplicationID, secret.ID, version)
	}
	if err != nil {
		return "", 0, err
	}

	plain, stale, err := v.Box.OpenStale(stored.Ciphertext, associatedData(stored.ID))
	if err != nil {
		return "", 0, errors.New("could not decrypt the secret")
	}

	if stale {
		if resealed, err := v.Box.Seal(plain, associatedData(stored.ID)); err == nil {
			_, _ = v.Resealer.Reseal(ctx, secret.ApplicationID, secret.ID, entities.SecretVersion{ID: stored.ID, Ciphertext: resealed})
		}
	}

	return string(plain), stored.Version, nil
}
