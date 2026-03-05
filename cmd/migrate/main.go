package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/tonypk/aigonhr/internal/config"
)

func main() {
	cmd := flag.String("cmd", "up", "goose command: up, down, status, create")
	name := flag.String("name", "", "migration name (for create)")
	flag.Parse()

	cfg := config.Load()
	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = "db/migrations"
	}

	db, err := sql.Open("pgx", cfg.Postgres.DSN())
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	switch *cmd {
	case "up":
		if err := goose.Up(db, migrationsDir); err != nil {
			log.Fatalf("goose up: %v", err)
		}
	case "down":
		if err := goose.Down(db, migrationsDir); err != nil {
			log.Fatalf("goose down: %v", err)
		}
	case "status":
		if err := goose.Status(db, migrationsDir); err != nil {
			log.Fatalf("goose status: %v", err)
		}
	case "create":
		if *name == "" {
			log.Fatal("migration name required: -name <name>")
		}
		if err := goose.Create(db, migrationsDir, *name, "sql"); err != nil {
			log.Fatalf("goose create: %v", err)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", *cmd)
		os.Exit(1)
	}
}
