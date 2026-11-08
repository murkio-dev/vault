package entities

import (
	"errors"
	"uuid"
)

// ErrMissingParam é o erro de um repositório que recebeu o id de isolamento
// (aplicação, dono, segredo) zerado. É falha de programação, não do cliente: o
// use case responde 500, e o repositório não toca o banco. Zero nunca significa
// "sem filtro".
var ErrMissingParam = errors.New("repository parameter is missing")

// Require devolve ErrMissingParam se qualquer id for zero. Todo método de
// repositório que filtra por aplicação, dono ou segredo chama na primeira linha.
func Require(ids ...uuid.UUID) error {
	for _, id := range ids {
		if id == (uuid.UUID{}) {
			return ErrMissingParam
		}
	}
	return nil
}
