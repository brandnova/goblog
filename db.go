package main

import (
    "log"
    _ "github.com/mattn/go-sqlite3"
    "github.com/jmoiron/sqlx"
)

var db *sqlx.DB

func initDB() {
    var err error
    db, err = sqlx.Connect("sqlite3", "blog.db")
    if err != nil {
        log.Fatal("Could not connect to database:", err)
    }

    schema := `
    CREATE TABLE IF NOT EXISTS users (
        id            INTEGER PRIMARY KEY AUTOINCREMENT,
        username      TEXT NOT NULL UNIQUE,
        email         TEXT NOT NULL UNIQUE,
        password_hash TEXT NOT NULL,
        created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
    );

    CREATE TABLE IF NOT EXISTS posts (
        id           INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id      INTEGER NOT NULL REFERENCES users(id),
        title        TEXT NOT NULL,
        slug         TEXT NOT NULL UNIQUE,
        body         TEXT NOT NULL,
        cover_image  TEXT DEFAULT '',
        status       TEXT DEFAULT 'draft',   -- 'draft' or 'published'
        created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
    );

    CREATE TABLE IF NOT EXISTS tags (
        id   INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL UNIQUE
    );

    -- This is a join table, like a Django ManyToManyField
    CREATE TABLE IF NOT EXISTS post_tags (
        post_id INTEGER REFERENCES posts(id) ON DELETE CASCADE,
        tag_id  INTEGER REFERENCES tags(id)  ON DELETE CASCADE,
        PRIMARY KEY (post_id, tag_id)
    );

    CREATE TABLE IF NOT EXISTS sessions (
        token      TEXT PRIMARY KEY,
        user_id    INTEGER NOT NULL REFERENCES users(id),
        expires_at DATETIME NOT NULL
    );`

    db.MustExec(schema)
    log.Println("Database ready.")
}