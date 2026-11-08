package sqlite

import (
	"context"
	"database/sql"
)

type Executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type txKey struct{}

// Conn devolve a transação aberta no ctx por Transactor.Do, ou o db se não
// houver. Todo repositório usa Conn em vez do db direto: é isso que faz a
// mudança, o registro de auditoria e o resto da operação entrarem na mesma
// transação sem o use case conhecer SQL. Com a conexão única do pool, uma
// consulta feita pelo db enquanto há uma transação aberta travaria.
func Conn(ctx context.Context, db *sql.DB) Executor {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return db
}

type Transactor struct {
	db *sql.DB
}

func NewTransactor(db *sql.DB) Transactor {
	return Transactor{db: db}
}

// Do chama fn com um ctx que carrega a transação aberta, e a confirma se fn
// devolver nil. Se fn devolver erro ou der panic, a transação é desfeita (no
// caso do panic, o panic segue depois do rollback, sem deixar a conexão única
// presa numa transação aberta). Chamada dentro de outra Do, reaproveita a
// transação que já existe e quem abriu é quem fecha: abrir outra travaria,
// porque o pool tem uma conexão só. Exemplo: o use case roda a mudança e a
// auditoria dentro de Do, e a falha de qualquer uma desfaz as duas.
func (t Transactor) Do(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if _, nested := ctx.Value(txKey{}).(*sql.Tx); nested {
		return fn(ctx)
	}

	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback()
			panic(recovered)
		}
	}()

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}
