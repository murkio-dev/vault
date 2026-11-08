package entities

import "uuid"

type AuthorizeInput struct {
	Requester      ApiHeader
	Application    uuid.UUID
	Action         Action
	Owner          uuid.UUID
	OwnerMustExist bool
}

type AuthorizeOutput struct {
	Member Member
	Owner  uuid.UUID
	Error  *Error
}
