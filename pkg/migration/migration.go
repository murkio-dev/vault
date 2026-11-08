package migration

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"
)

type Runner struct {
	db *sql.DB
	fs embed.FS
}

func New(db *sql.DB, fs embed.FS) *Runner {
	return &Runner{
		db: db,
		fs: fs,
	}
}

func (r *Runner) Run(ctx context.Context, dir string) error {
	if err := r.initHistoryTable(ctx); err != nil {
		return fmt.Errorf("failed to init schema_migrations table: %w", err)
	}

	entries, err := r.fs.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read embedded dir '%s': %w", dir, err)
	}

	var files []string
	for _, entry := range entries {
		// Só as .up.sql rodam no boot. Uma .down.sql ordena antes da .up.sql do mesmo
		// número, então sem este filtro ela seria executada e apagaria a tabela.
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			files = append(files, entry.Name())
		}
	}

	sort.Strings(files)

	for _, fileName := range files {
		applied, err := r.isApplied(ctx, fileName)
		if err != nil {
			return fmt.Errorf("failed to check migration '%s': %w", fileName, err)
		}
		if applied {
			continue
		}

		filePath := path.Join(dir, fileName)
		content, err := r.fs.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read migration file '%s': %w", filePath, err)
		}

		if err := r.executeMigration(ctx, fileName, string(content)); err != nil {
			return fmt.Errorf("failed to execute migration '%s': %w", fileName, err)
		}
	}

	return nil
}

func (r *Runner) initHistoryTable(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at DATETIME NOT NULL
	);`
	_, err := r.db.ExecContext(ctx, query)
	return err
}

func (r *Runner) isApplied(ctx context.Context, version string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, version).Scan(&exists)
	return exists, err
}

func (r *Runner) executeMigration(ctx context.Context, version, sqlContent string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, sqlContent); err != nil {
		return err
	}

	recordQuery := `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`
	if _, err := tx.ExecContext(ctx, recordQuery, version, time.Now()); err != nil {
		return err
	}

	return tx.Commit()
}
