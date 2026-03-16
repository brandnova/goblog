package models

import (
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

// User maps to the users table.
// Django parallel: a Model class with CharField, EmailField etc.
type User struct {
	ID           int       `db:"id"`
	Username     string    `db:"username"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	FirstName    string    `db:"first_name"`
	LastName     string    `db:"last_name"`
	Bio          string    `db:"bio"`
	CreatedAt    time.Time `db:"created_at"`
}

// DisplayName returns the user's full name if set, otherwise their username.
// Called in templates as {{ .User.DisplayName }} — just like a Django model method.
func (u *User) DisplayName() string {
	if u.FirstName != "" || u.LastName != "" {
		name := u.FirstName
		if u.LastName != "" {
			if name != "" {
				name += " "
			}
			name += u.LastName
		}
		return name
	}
	return u.Username
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

// UpdateProfile updates a user's display name, bio, and email.
// Email is checked for uniqueness against other users before saving.
// Django parallel: user.save() after modifying fields, with form-level validation.
func UpdateProfile(db *sqlx.DB, userID int, firstName, lastName, bio, email string) error {
	// Check the email isn't already taken by a different account
	var existingID int
	err := db.Get(&existingID, "SELECT id FROM users WHERE email = $1", email)
	if err == nil && existingID != userID {
		return errors.New("that email address is already in use")
	}

	_, err = db.Exec(`
		UPDATE users
		SET first_name = $1, last_name = $2, bio = $3, email = $4
		WHERE id = $5
	`, firstName, lastName, bio, email, userID)
	return err
}

// UpdatePassword changes a user's password after verifying their current one.
// Django parallel: user.set_password() gated by check_password()
func UpdatePassword(db *sqlx.DB, userID int, currentPassword, newPassword string) error {
	// Fetch the current hash to verify against
	var hash string
	if err := db.Get(&hash, "SELECT password_hash FROM users WHERE id = $1", userID); err != nil {
		return errors.New("user not found")
	}

	if !CheckPassword(currentPassword, hash) {
		return errors.New("current password is incorrect")
	}

	newHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	_, err = db.Exec("UPDATE users SET password_hash = $1 WHERE id = $2", newHash, userID)
	return err
}