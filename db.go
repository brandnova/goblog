package main

import (
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var db *sqlx.DB

func initDB() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	var err error
	db, err = sqlx.Connect("postgres", dsn)
	if err != nil {
		log.Fatal("Could not connect to database:", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id            SERIAL PRIMARY KEY,
		username      TEXT NOT NULL UNIQUE,
		email         TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		created_at    TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS posts (
		id           SERIAL PRIMARY KEY,
		user_id      INTEGER NOT NULL REFERENCES users(id),
		title        TEXT NOT NULL,
		slug         TEXT NOT NULL UNIQUE,
		body         TEXT NOT NULL,
		cover_image  TEXT DEFAULT '',
		status       TEXT DEFAULT 'draft',
		created_at   TIMESTAMPTZ DEFAULT NOW(),
		updated_at   TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS tags (
		id   SERIAL PRIMARY KEY,
		name TEXT NOT NULL UNIQUE
	);

	CREATE TABLE IF NOT EXISTS post_tags (
		post_id INTEGER REFERENCES posts(id) ON DELETE CASCADE,
		tag_id  INTEGER REFERENCES tags(id)  ON DELETE CASCADE,
		PRIMARY KEY (post_id, tag_id)
	);

	CREATE TABLE IF NOT EXISTS sessions (
		token      TEXT PRIMARY KEY,
		user_id    INTEGER NOT NULL REFERENCES users(id),
		expires_at TIMESTAMPTZ NOT NULL
	);`

	db.MustExec(schema)
	log.Println("Database ready.")
}
