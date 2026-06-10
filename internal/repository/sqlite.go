// Package repository: sqlite.go é a CAIXA CINZA — a implementação concreta (SQL e só SQL).
// O service nunca importa este arquivo; só conhece as interfaces de repository.go.
//
// Os métodos do *SQLiteRepo estão divididos por domínio em sqlite_*.go (mesmo pacote):
// questionnaire, exercise, block, conditioning. Aqui ficam só a struct, New e helpers.
package repository

import (
	"database/sql"
	"strings"
)

// SQLiteRepo implementa todos os contratos de repository sobre um *sql.DB.
type SQLiteRepo struct {
	db *sql.DB
}

// New devolve um repositório respaldado pelo banco já aberto.
func New(db *sql.DB) *SQLiteRepo {
	return &SQLiteRepo{db: db}
}

// isUniqueConstraint detecta violação de UNIQUE no SQLite (sem importar o driver diretamente).
func isUniqueConstraint(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
