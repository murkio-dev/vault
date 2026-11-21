package services

import (
	"net/http"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/httpx"
)

// ParseOwner lê de quem são as chaves, na query: application_id (obrigatório)
// e member_id (um UUID ou "me"; ausente vale "me", o membro de quem chama).
// Exemplo: "/api/v1/api-keys?application_id=<uuid>" lista as minhas chaves.
func ParseOwner(r *http.Request) (application, member uuid.UUID, e *entities.Error) {
	query := r.URL.Query()
	if application, e = queryUUID(query.Get("application_id"), "application_id"); e != nil {
		return
	}

	raw := query.Get("member_id")
	if raw == "" || raw == "me" {
		return application, entities.SelfOwner, nil
	}
	member, e = queryUUID(raw, "member_id")
	return
}

// ParseByID lê o dono (ParseOwner) e o {id} da chave no caminho.
func ParseByID(r *http.Request) (application, member, id uuid.UUID, e *entities.Error) {
	if application, member, e = ParseOwner(r); e != nil {
		return
	}
	id, e = httpx.PathUUID(r, "id")
	return
}

type createBody struct {
	Name      string           `json:"name"`
	Type      entities.KeyType `json:"type"`
	ExpiresAt *time.Time       `json:"expires_at"`
}

type CreateData struct {
	Application uuid.UUID
	Member      uuid.UUID
	Name        string
	Type        entities.KeyType
	ExpiresAt   *time.Time
}

func ParseCreate(r *http.Request) (CreateData, *entities.Error) {
	application, member, e := ParseOwner(r)
	if e != nil {
		return CreateData{}, e
	}
	body, e := httpx.BindOne[createBody](r)
	if e != nil {
		return CreateData{}, e
	}
	return CreateData{Application: application, Member: member, Name: body.Name, Type: body.Type, ExpiresAt: body.ExpiresAt}, nil
}

type updateBody struct {
	Name string `json:"name"`
}

func ParseUpdate(r *http.Request) (application, member, id uuid.UUID, name string, e *entities.Error) {
	if application, member, id, e = ParseByID(r); e != nil {
		return
	}
	body, e := httpx.BindOne[updateBody](r)
	if e != nil {
		return
	}
	return application, member, id, body.Name, nil
}

func queryUUID(raw, name string) (uuid.UUID, *entities.Error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == (uuid.UUID{}) {
		invalid := entities.ValidationError("The query parameter "+name+" is invalid.",
			entities.InvalidParam{Field: name, Reason: "must be a valid, non-empty uuid"})
		return uuid.UUID{}, &invalid
	}
	return id, nil
}
