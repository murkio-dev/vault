// Package sqlite abre o banco do serviço: um arquivo SQLite em WAL, com uma
// conexão só.
//
// O SQLite aceita um escritor por vez. Com uma conexão, as chamadas fazem fila
// no database/sql, em ordem e sem SQLITE_BUSY. Toda transação abre com BEGIN
// IMMEDIATE (_txlock=immediate): ela pega o lock de escrita no começo e nunca
// falha no meio por upgrade de lock. Por isso, dentro de uma transação, toda
// consulta tem de usar o *sql.Tx e nunca o *sql.DB, que ficaria esperando a
// única conexão.
package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const BusyTimeout = 10 * time.Second

func New(path string) (*sql.DB, error) {
	if path == "" {
		return nil, errors.New("sqlite: empty database path")
	}

	if !isMemory(path) {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return nil, fmt.Errorf("sqlite: creating the database directory: %w", err)
		}

		// O banco nasce legível só pelo dono: ele guarda os segredos cifrados e a
		// auditoria. O SQLite cria o -wal e o -shm com o modo do arquivo principal.
		if err := createPrivate(path); err != nil {
			return nil, fmt.Errorf("sqlite: creating the database file: %w", err)
		}
	}

	db, err := sql.Open("sqlite", DSN(path))
	if err != nil {
		return nil, fmt.Errorf("sqlite: opening the database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: connecting to the database: %w", err)
	}

	return db, nil
}

// DSN monta a conexão do modernc.org/sqlite. As datas são gravadas em UTC no
// formato do SQLite com fração variável: em UTC, a ordem do texto é a ordem do
// tempo, então comparações e ORDER BY nas colunas de data funcionam direto no
// SQL. _texttotime lê como time.Time também as expressões sem tipo declarado,
// _txlock=immediate faz toda transação pegar o lock de escrita no começo, e o
// WAL só vale para arquivo (banco em memória não tem journal).
func DSN(path string) string {
	params := url.Values{}
	params.Set("_time_format", "sqlite")
	params.Set("_timezone", "UTC")
	params.Set("_texttotime", "1")
	params.Set("_txlock", "immediate")
	params.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", BusyTimeout.Milliseconds()))
	params.Add("_pragma", "foreign_keys(1)")
	params.Add("_pragma", "temp_store(memory)")

	if !isMemory(path) {
		params.Add("_pragma", "journal_mode(WAL)")
		params.Add("_pragma", "synchronous(NORMAL)")
	}

	prefix := "file:"
	if strings.HasPrefix(path, "file:") {
		prefix = ""
	}
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}

	return prefix + path + separator + params.Encode()
}

func createPrivate(path string) error {
	if strings.HasPrefix(path, "file:") {
		return nil
	}

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	return file.Close()
}

func isMemory(path string) bool {
	return path == ":memory:" || strings.Contains(path, "mode=memory") || strings.HasPrefix(path, "file::memory:")
}

// IsUniqueViolation diz se err é a recusa de uma restrição UNIQUE ou PRIMARY
// KEY. Os repositórios devolvem o erro do driver como veio, e o use case usa
// esta função para traduzi-lo no conflito do recurso (409) em vez de 500.
// Entrada: qualquer erro, inclusive nil (devolve false). Exemplo: inserir um
// segundo segredo com o mesmo nome na tabela do membro devolve true, enquanto
// um CHECK violado devolve false (use IsConstraintViolation para esse caso).
func IsUniqueViolation(err error) bool {
	code, ok := errorCode(err)
	return ok && (code == sqlite3.SQLITE_CONSTRAINT_UNIQUE || code == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY)
}

// IsConstraintViolation diz se err é a recusa de qualquer restrição do banco:
// UNIQUE, CHECK, FOREIGN KEY, NOT NULL ou um trigger com RAISE. A trilha de
// auditoria depende disso, porque o trigger append-only recusa UPDATE e DELETE
// com RAISE(ABORT), e os testes provam essa recusa por aqui. Entrada: qualquer
// erro, inclusive nil (devolve false). Exemplo: DELETE em audit_logs devolve true.
func IsConstraintViolation(err error) bool {
	code, ok := errorCode(err)
	return ok && code&0xff == sqlite3.SQLITE_CONSTRAINT
}

func errorCode(err error) (int, bool) {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return 0, false
	}
	return sqliteErr.Code(), true
}
