package entities

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"
)

const MaxSecretValueBytes = 32 << 10

var secretName = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,127}$`)

type SecretType string

const (
	SecretString      SecretType = "string"
	SecretPassword    SecretType = "password"
	SecretURL         SecretType = "url"
	SecretDatabaseURI SecretType = "database_uri"
	SecretAPIKey      SecretType = "api_key"
	SecretJWTKey      SecretType = "jwt_key"
	SecretPasetoKey   SecretType = "paseto_key"
)

var secretTypes = map[SecretType]bool{
	SecretString: true, SecretPassword: true, SecretURL: true, SecretDatabaseURI: true,
	SecretAPIKey: true, SecretJWTKey: true, SecretPasetoKey: true,
}

func (t SecretType) Valid() bool {
	return secretTypes[t]
}

type SecretSource string

const (
	SourceStatic    SecretSource = "static"
	SourceGenerated SecretSource = "generated"
	SourceReference SecretSource = "reference"
	SourceDynamic   SecretSource = "dynamic"
)

type Secret struct {
	ID                uuid.UUID    `json:"id"`
	ApplicationID     uuid.UUID    `json:"application_id"`
	MemberID          uuid.UUID    `json:"member_id"`
	Name              string       `json:"name"`
	Type              SecretType   `json:"type"`
	Source            SecretSource `json:"source"`
	ReferenceSecretID *uuid.UUID   `json:"reference_secret_id"`
	Version           int          `json:"version"`
	CreatedBy         uuid.UUID    `json:"created_by"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         *time.Time   `json:"updated_at"`
}

type SecretVersion struct {
	ID         uuid.UUID `json:"-"`
	Version    int       `json:"version"`
	CreatedBy  uuid.UUID `json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
	Ciphertext []byte    `json:"-"`
}

// SecretWrite é o que o repositório de segredos recebe para gravar: os
// metadados e, para um segredo com valor, a versão já cifrada. O valor em claro
// nunca chega ao repositório.
type SecretWrite struct {
	Secret
	VersionID  uuid.UUID
	Ciphertext []byte
	// Author é quem grava a versão (o ator). Vazio na criação vale o criador.
	Author uuid.UUID
}

// MaxSecretsPerTable é quantos segredos cabem na tabela de um membro. O
// runtime devolve a tabela inteira numa resposta, então ela precisa ter teto.
const MaxSecretsPerTable = 100

// MaxVersionsListed é quantas versões a listagem devolve (as mais novas).
const MaxVersionsListed = 500

type SecretDraft struct {
	Name              string       `json:"name"`
	Type              SecretType   `json:"type"`
	Source            SecretSource `json:"source"`
	Value             string       `json:"value"`
	ReferenceSecretID *uuid.UUID   `json:"reference_secret_id"`
}

type RevealedSecret struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Version int       `json:"version"`
	Value   string    `json:"value"`
}

type RuntimeVariable struct {
	Name    string     `json:"name"`
	Type    SecretType `json:"type"`
	Version int        `json:"version"`
	Value   string     `json:"value"`
}

func NewSecret(applicationID, memberID uuid.UUID, name string, secretType SecretType, source SecretSource, reference *uuid.UUID, createdBy uuid.UUID) Secret {
	return Secret{
		ID:                uuid.NewV7(),
		ApplicationID:     applicationID,
		MemberID:          memberID,
		Name:              name,
		Type:              secretType,
		Source:            source,
		ReferenceSecretID: reference,
		CreatedBy:         createdBy,
		CreatedAt:         time.Now().UTC(),
	}
}

func ValidateSecretName(name string) error {
	if !secretName.MatchString(name) {
		return errors.New("must match ^[A-Z][A-Z0-9_]{0,127}$")
	}
	return nil
}

// ValidateSecretValue confere o valor contra o tipo do segredo: não vazio,
// até MaxSecretValueBytes, texto UTF-8 sem byte nulo e, para url e
// database_uri, uma URL com esquema e host e sem quebra de linha. Os erros
// dizem o que esperar e NUNCA repetem o valor recebido, porque ele é o segredo.
// Exemplo: (SecretURL, "segredo") devolve "must be a URL with scheme and host".
// Os outros tipos aceitam qualquer texto, e PEM (com quebras) vale para chaves.
func ValidateSecretValue(secretType SecretType, value string) error {
	if value == "" {
		return errors.New("must not be empty")
	}
	if len(value) > MaxSecretValueBytes {
		return errors.New("is too large")
	}
	if !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
		return errors.New("must be valid text")
	}

	switch secretType {
	case SecretString, SecretPassword, SecretAPIKey, SecretJWTKey, SecretPasetoKey:
		return nil
	case SecretURL, SecretDatabaseURI:
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || strings.ContainsAny(value, "\r\n") {
			return errors.New("must be a URL with scheme and host")
		}
		return nil
	}

	return errors.New("unknown type")
}
