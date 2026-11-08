package entities

import (
	"fmt"
	"net/http"
	"strings"
	"uuid"
)

// RFC 9457 (problem details):
//
//	{
//	  "type": "https://murkio.com/en-us/docs/vault/errors/#validation",
//	  "title": "Validation error",
//	  "status": 400,
//	  "detail": "The field name is invalid.",
//	  "instance": "/v1/applications",
//	  "invalidParams": [
//	    {"field": "name", "reason": "application name cannot obtain '*' and spaces.", "value": "my app *"}
//	  ]
//	}
type Error struct {
	Type          string `json:"type"`
	Title         string `json:"title"`
	Status        int    `json:"status"`
	Detail        string `json:"detail,omitempty"`
	Instance      string `json:"instance,omitempty"`
	InvalidParams []any  `json:"invalid_params"`
}

type InvalidParam struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
	Value  any    `json:"value,omitempty"`
}

const errorsDocsURL = "https://murkio.com/en-us/docs/vault/errors/"

const (
	ErrTypeValidation = errorsDocsURL + "#validation"
	ErrTypeNotFound   = errorsDocsURL + "#not-found"
	ErrTypeConflict   = errorsDocsURL + "#conflict"
	ErrTypeInternal   = errorsDocsURL + "#internal"

	ErrTypeUnauthorized = errorsDocsURL + "#unauthorized"
	ErrTypeForbidden    = errorsDocsURL + "#forbidden"
	ErrTypeUnavailable  = errorsDocsURL + "#unavailable"
)

func (e Error) HTTPStatus() int {
	if e.Status == 0 {
		return http.StatusInternalServerError
	}
	return e.Status
}

func NewError(errType, title string, status int, detail string, params ...InvalidParam) Error {
	invalid := make([]any, 0, len(params))
	for _, p := range params {
		invalid = append(invalid, p)
	}
	return Error{
		Type:          errType,
		Title:         title,
		Status:        status,
		Detail:        detail,
		InvalidParams: invalid,
	}
}

func validationError(detail string, params ...InvalidParam) Error {
	return NewError(ErrTypeValidation, "Validation error", http.StatusBadRequest, detail, params...)
}

func reason(err error) string {
	if err == nil {
		return "invalid value"
	}
	return err.Error()
}

func InternalServerError(_ error, _ *string) Error {
	return NewError(ErrTypeInternal, "Internal server error", http.StatusInternalServerError,
		"An unexpected error occurred. Try again later.")
}

func InvalidRequestBodyError(err error) Error {
	return validationError("The request body is invalid.",
		InvalidParam{Field: "body", Reason: reason(err)})
}

func InvalidApplicationIDFormatError(err error) Error {
	return validationError("The application id has an invalid format.",
		InvalidParam{Field: "id", Reason: reason(err)})
}

func InvalidApplicationIDError(id uuid.UUID) Error {
	return validationError("The application id is invalid.",
		InvalidParam{Field: "id", Reason: "must be a valid, non-empty uuid", Value: id.String()})
}

func DuplicateApplicationIDError(id uuid.UUID) Error {
	return validationError("The application id was sent more than once.",
		InvalidParam{Field: "id", Reason: "duplicated in the request", Value: id.String()})
}

func ApplicationNotFoundError(_ uuid.UUID) Error {
	return NotFoundError("Resource")
}

func ApplicationAlreadyExistsError(name string) Error {
	return NewError(ErrTypeConflict, "Application already exists", http.StatusConflict,
		fmt.Sprintf("The application %q already exists.", name),
		InvalidParam{Field: "name", Reason: "already exists", Value: name})
}

func InvalidApplicationNameError(err error, name *string) Error {
	p := InvalidParam{Field: "name", Reason: reason(err)}
	if name != nil {
		p.Value = *name
	}
	return validationError("The field name is invalid.", p)
}

func InvalidPageError(err error) Error {
	return validationError("The query parameter page is invalid.",
		InvalidParam{Field: "page", Reason: reason(err)})
}

func InvalidLimitError(err error) Error {
	return validationError("The query parameter limit is invalid.",
		InvalidParam{Field: "limit", Reason: reason(err)})
}

// UnauthorizedError é a resposta a toda credencial ausente, inválida, vencida
// ou revogada. É sempre a mesma, sem motivo: dizer qual foi ajudaria quem tenta
// forjar ou adivinhar um token. Quem chega aqui não se identificou, então não
// há autor para a trilha de auditoria. Exemplo: um JWT com assinatura errada e
// um token vt_ revogado devolvem exatamente o mesmo corpo.
func UnauthorizedError() Error {
	return NewError(ErrTypeUnauthorized, "Unauthorized", http.StatusUnauthorized,
		"A valid credential is required.")
}

func ForbiddenError() Error {
	return NewError(ErrTypeForbidden, "Forbidden", http.StatusForbidden,
		"You are not allowed to do this.")
}

// NotFoundError é a resposta a um recurso que não existe ou que não é visível
// para quem pediu: as duas situações são indistinguíveis de propósito, para
// quem não é membro não descobrir o que existe. O recurso entra só no título
// (Resource, Application...), nunca o id pedido. Exemplo: um id de aplicação
// alheia e um id inventado recebem o mesmo 404, inclusive o mesmo corpo.
func NotFoundError(resource string) Error {
	return NewError(ErrTypeNotFound, resource+" not found", http.StatusNotFound,
		"The "+strings.ToLower(resource)+" was not found.")
}

func ConflictError(title, detail string, params ...InvalidParam) Error {
	return NewError(ErrTypeConflict, title, http.StatusConflict, detail, params...)
}

func ValidationError(detail string, params ...InvalidParam) Error {
	return validationError(detail, params...)
}
