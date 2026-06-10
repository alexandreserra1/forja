#!/usr/bin/env bash
# Sobe o servidor Go para o e2e com um banco TEMPORÁRIO recém-semeado, de forma que cada execução
# comece limpa (sem bloco ativo de uma rodada anterior) e o teste seja determinístico.
set -euo pipefail

# Raiz do repositório (este script vive em web/e2e/): é de lá que `go run ./cmd/server` enxerga go.mod.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

DB="$(mktemp -d)/e2e.db"
sqlite3 "$DB" < db/schema.sql
sqlite3 "$DB" < db/seed.sql

echo "e2e: backend Go com db efêmero em $DB"
exec go run ./cmd/server -db "$DB" -addr :8080
