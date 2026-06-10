// Comando server: o ÚNICO lugar que conhece todas as peças concretas.
// Abre o banco, monta repository -> service -> handler (injeção de dependência)
// e sobe o HTTP. As camadas internas nunca se conhecem por baixo do contrato.
package main

import (
	"database/sql"
	"flag"
	"log"
	"net/http"

	_ "github.com/mattn/go-sqlite3" // registra o driver "sqlite3" (blank import fica AQUI)

	"treino/internal/handler"
	"treino/internal/repository"
	"treino/internal/service"
)

func main() {
	dbPath := flag.String("db", "cfit.db", "caminho do arquivo SQLite")
	addr := flag.String("addr", ":8080", "endereço de escuta HTTP")
	flag.Parse()

	// foreign_keys=on no DSN: o PRAGMA do schema é por conexão; garantimos aqui também.
	db, err := sql.Open("sqlite3", *dbPath+"?_foreign_keys=on")
	if err != nil {
		log.Fatalf("abrir banco: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("banco inacessível (%s): %v", *dbPath, err)
	}

	// Fiação: concreto -> contrato -> cérebro -> borda.
	repo := repository.New(db)
	svc := service.New(repo)
	h := handler.New(svc)

	log.Printf("cfit ouvindo em %s (db: %s)", *addr, *dbPath)
	if err := http.ListenAndServe(*addr, handler.CORS(h.Routes())); err != nil {
		log.Fatalf("servidor parou: %v", err)
	}
}
