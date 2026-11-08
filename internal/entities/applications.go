package entities

import (
	"errors"
	"regexp"
	"time"
	"uuid"
)

var applicationName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$`)

type Application struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	CreatedBy uuid.UUID  `json:"created_by"`
	UpdatedAt *time.Time `json:"updated_at"`
	CreatedAt time.Time  `json:"created_at"`
}

func NewApplication(name string, createdBy uuid.UUID) (Application, error) {

	var output Application

	if !applicationName.MatchString(name) {
		return output, errors.New("Invalid name.")
	}

	output = Application{
		ID:        uuid.NewV7(),
		Name:      name,
		CreatedBy: createdBy,
		UpdatedAt: nil,
		CreatedAt: time.Now().UTC(),
	}

	return output, nil

}

func (a *Application) Validate() error {
	if !applicationName.MatchString(a.Name) {
		return errors.New("Invalid name.")
	}
	return nil
}
