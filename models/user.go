package models

import (
	"time"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

// User maps to the users table.
// The `db` struct tags tell sqlx which column maps to which field.
// Django parallel: a Model class with CharField, EmailField etc.
type User struct {
	ID           int       `db:"id"`
	Username     string    `db:"username"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	CreatedAt    time.Time `db:"created_at"`
}

// HashPassword hashes a plain-text password using bcrypt.
// Cost factor 14 = ~2^14 iterations — slow enough to resist brute force,
// fast enough for a login request.
// Django parallel: make_password() / PBKDF2PasswordHasher
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

// CheckPassword checks a plain-text password against a stored bcrypt hash.
// Django parallel: check_password()
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// CreateUser inserts a new user into the database with a hashed password.
// Django parallel: User.objects.create_user()
func CreateUser(db *sqlx.DB, username, email, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		"INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3)",
		username, email, hash,
	)
	return err
}

// GetUserByEmail fetches a user by email address.
// Django parallel: User.objects.get(email=email)
func GetUserByEmail(db *sqlx.DB, email string) (*User, error) {
	user := &User{}
	err := db.Get(user, "SELECT * FROM users WHERE email = $1", email)
	return user, err
}

// GetUserByID fetches a user by their primary key.
// Django parallel: User.objects.get(pk=id)
func GetUserByID(db *sqlx.DB, id int) (*User, error) {
	user := &User{}
	err := db.Get(user, "SELECT * FROM users WHERE id = $1", id)
	return user, err
}

// GetUserByUsername fetches a user by their username.
// Django parallel: User.objects.get(username=username)
func GetUserByUsername(db *sqlx.DB, username string) (*User, error) {
	user := &User{}
	err := db.Get(user, "SELECT * FROM users WHERE username = $1", username)
	return user, err
}