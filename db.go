package main

import (
	"log"
	"os"
	"time"

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
		first_name    TEXT DEFAULT '',
		last_name     TEXT DEFAULT '',
		bio           TEXT DEFAULT '',
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
	);

	CREATE TABLE IF NOT EXISTS bookmarks (
		user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		post_id    INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		PRIMARY KEY (user_id, post_id)
	);

	CREATE TABLE IF NOT EXISTS reactions (
		user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
		PRIMARY KEY (user_id, post_id)
	);`

	db.MustExec(schema)

	// Safe migrations — ADD COLUMN IF NOT EXISTS is a no-op if already present.
	// Run on every startup so new deployments pick up schema changes automatically.
	migrations := []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS first_name TEXT DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS last_name  TEXT DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS bio        TEXT DEFAULT ''`,
	}
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			log.Printf("Migration warning: %v\n", err)
		}
	}

	// Session cleanup goroutine — runs every 24 hours and removes all
	// expired sessions from the table. Without this, the sessions table
	// grows indefinitely since every login writes a new row.
	//
	// Runs in the background so it never blocks startup or requests.
	// The 'go' keyword means main() returns immediately and this loop
	// continues for the lifetime of the process.
	go func() {
		for {
			result, err := db.Exec("DELETE FROM sessions WHERE expires_at < NOW()")
			if err != nil {
				log.Printf("Session cleanup error: %v\n", err)
			} else {
				n, _ := result.RowsAffected()
				if n > 0 {
					log.Printf("Session cleanup: removed %d expired session(s)\n", n)
				}
			}
			time.Sleep(24 * time.Hour)
		}
	}()

	log.Println("Database ready.")
}