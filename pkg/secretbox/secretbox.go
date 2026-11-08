// Package secretbox cifra os valores dos segredos em repouso com AES-256-GCM.
//
// O Box é um keyring: a chave ativa (ENCRYPTION_KEY) cifra tudo o que é novo, e
// as antigas (ENCRYPTION_KEYS_PREVIOUS) só abrem o que já foi cifrado com
// elas. O valor cifrado não guarda qual chave o cifrou: o Open tenta a ativa e
// depois as antigas, e o tag do GCM garante que a chave errada não abre.
//
// O associatedData amarra o ciphertext ao seu contexto (ex.:
// "secret_version:<id>"): um valor copiado para outra linha não abre.
// Para trocar a chave, ponha a nova em ENCRYPTION_KEY e a antiga em
// ENCRYPTION_KEYS_PREVIOUS; o OpenStale diz se o valor precisa ser recifrado.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
)

const KeySize = 32

var (
	ErrInvalidKey    = errors.New("secretbox: invalid encryption key")
	ErrInvalidSealed = errors.New("secretbox: invalid sealed value")
)

type Box struct {
	active   cipher.AEAD
	previous []cipher.AEAD
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != KeySize {
		return nil, ErrInvalidKey
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	return cipher.NewGCM(block)
}

func New(active []byte, previous ...[]byte) (*Box, error) {
	aead, err := newAEAD(active)
	if err != nil {
		return nil, err
	}

	box := &Box{active: aead}
	for i, key := range previous {
		if subtle.ConstantTimeCompare(key, active) == 1 {
			return nil, fmt.Errorf("%w: previous key %d is the same as the active one", ErrInvalidKey, i+1)
		}

		old, err := newAEAD(key)
		if err != nil {
			return nil, fmt.Errorf("%w: previous key %d must be %d bytes", ErrInvalidKey, i+1, KeySize)
		}
		box.previous = append(box.previous, old)
	}

	return box, nil
}

// FromBase64 lê a ENCRYPTION_KEY e as ENCRYPTION_KEYS_PREVIOUS, todas em
// base64 de 32 bytes. O serviço não sobe com uma chave inválida: a mensagem
// diz qual variável corrigir (e a posição, nas antigas) e nunca repete o valor.
// Uma antiga igual à ativa é recusada, porque é rotação mal feita.
// Exemplo: FromBase64(nova, []string{antiga}) cifra com a nova e ainda abre o
// que foi cifrado com a antiga.
func FromBase64(active string, previous []string) (*Box, error) {
	activeKey, err := base64.StdEncoding.DecodeString(active)
	if err != nil || len(activeKey) != KeySize {
		return nil, fmt.Errorf("%w: ENCRYPTION_KEY must be %d bytes encoded in base64", ErrInvalidKey, KeySize)
	}

	previousKeys := make([][]byte, 0, len(previous))
	for i, encoded := range previous {
		key, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || len(key) != KeySize {
			return nil, fmt.Errorf("%w: ENCRYPTION_KEYS_PREVIOUS entry %d must be %d bytes encoded in base64", ErrInvalidKey, i+1, KeySize)
		}
		previousKeys = append(previousKeys, key)
	}

	return New(activeKey, previousKeys...)
}

func (b *Box) Seal(plain, associatedData []byte) ([]byte, error) {
	nonce := make([]byte, b.active.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	return b.active.Seal(nonce, nonce, plain, associatedData), nil
}

func (b *Box) Open(sealed, associatedData []byte) ([]byte, error) {
	plain, _, err := b.OpenStale(sealed, associatedData)
	return plain, err
}

// OpenStale abre o valor e diz se foi com uma chave antiga. O valor cifrado não
// guarda qual chave o cifrou: tenta a ativa e depois as antigas, na ordem, e o
// tag do GCM garante que a chave errada não abre. Quando devolve stale, o
// chamador deve recifrar o valor com a chave ativa (pkg/secretvalues faz isso
// na leitura), e depois disso a chave antiga pode sair. Valor curto demais ou
// que nenhuma chave abre devolve ErrInvalidSealed.
func (b *Box) OpenStale(sealed, associatedData []byte) ([]byte, bool, error) {
	size := b.active.NonceSize()
	if len(sealed) < size {
		return nil, false, ErrInvalidSealed
	}

	nonce, ciphertext := sealed[:size], sealed[size:]

	if plain, err := b.active.Open(nil, nonce, ciphertext, associatedData); err == nil {
		return plain, false, nil
	}

	for _, aead := range b.previous {
		if plain, err := aead.Open(nil, nonce, ciphertext, associatedData); err == nil {
			return plain, true, nil
		}
	}

	return nil, false, ErrInvalidSealed
}
