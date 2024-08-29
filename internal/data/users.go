package data

import (
	"cinevault.interimme.net/internal/validator"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"time"
)

// ErrDuplicateEmail is returned when a user tries to insert or update a user with an email that already exists in the database.
var ErrDuplicateEmail = errors.New("duplicate email")

// AnonymousUser represents a user who is not logged in.
var AnonymousUser = &User{}

// User represents an individual user in the application.
type User struct {
	ID        int64     `json:"id"`         // Unique identifier for the user.
	CreatedAt time.Time `json:"created_at"` // Timestamp of when the user was created.
	Name      string    `json:"name"`       // The user's name.
	Email     string    `json:"email"`      // The user's email address.
	Password  password  `json:"-"`          // The user's password, stored as a hashed value (not included in JSON output).
	Activated bool      `json:"activated"`  // Indicates whether the user's account is activated.
	Version   int       `json:"-"`          // Version number for optimistic concurrency control (not included in JSON output).
}

// IsAnonymous checks if the user is an anonymous user (not logged in).
func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}

// password represents a user's password with both plaintext (only temporarily) and hashed values.
type password struct {
	plaintext *string // The plaintext password, kept only temporarily during validation.
	hash      []byte  // The bcrypt hash of the password.
}

// UserModel wraps a sql.DB connection pool for performing operations on the users table.
type UserModel struct {
	DB *sql.DB
}

// Set hashes a plaintext password using bcrypt and stores both the plaintext (temporarily) and hashed password.
func (p *password) Set(plaintextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12) // Hash the password with bcrypt at cost 12.
	if err != nil {
		return err
	}
	p.plaintext = &plaintextPassword // Store the plaintext password temporarily.
	p.hash = hash                    // Store the hashed password.
	return nil
}

// Matches checks if a plaintext password matches the hashed password using bcrypt.
func (p *password) Matches(plaintextPassword string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintextPassword)) // Compare the plaintext and hashed password.
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil // Return false if the passwords do not match.
		default:
			return false, err // Return an error for other bcrypt errors.
		}
	}
	return true, nil // Return true if the passwords match.
}

// ValidateEmail checks if the email meets the application's validation criteria.
func ValidateEmail(v *validator.Validator, email string) {
	v.Check(email != "", "email", "must be provided")                                              // Check that the email is not empty.
	v.Check(validator.Matches(email, validator.EmailRX), "email", "must be a valid email address") // Check that the email matches a valid format.
}

// ValidatePasswordPlaintext checks if the plaintext password meets the application's security criteria.
func ValidatePasswordPlaintext(v *validator.Validator, password string) {
	v.Check(password != "", "password", "must be provided")                         // Check that the password is not empty.
	v.Check(len(password) >= 8, "password", "must be at least 8 bytes long")        // Check that the password is at least 8 characters long.
	v.Check(len(password) <= 72, "password", "must not be more than 72 bytes long") // Check that the password is not longer than 72 characters.
}

// ValidateUser validates the user's details and ensures the password hash is present.
func ValidateUser(v *validator.Validator, user *User) {
	v.Check(user.Name != "", "name", "must be provided")                           // Check that the name is not empty.
	v.Check(len(user.Name) <= 500, "name", "must not be more than 500 bytes long") // Check that the name is not too long.
	ValidateEmail(v, user.Email)                                                   // Validate the email format.

	if user.Password.plaintext != nil {
		ValidatePasswordPlaintext(v, *user.Password.plaintext) // Validate the plaintext password if it's provided.
	}

	if user.Password.hash == nil {
		panic("missing password hash for user") // Panic if the password hash is missing.
	}
}

// Insert adds a new user to the database, returning an error if the email already exists.
func (m UserModel) Insert(user *User) error {
	query := `
INSERT INTO users (name, email, password_hash, activated)
VALUES ($1, $2, $3, $4)
RETURNING id, created_at, version`

	args := []interface{}{user.Name, user.Email, user.Password.hash, user.Activated}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Execute the query and scan the returned id, created_at, and version into the user struct.
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.CreatedAt, &user.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail // Return a specific error if the email is already in use.
		default:
			return err // Return any other errors that occur.
		}
	}
	return nil
}

// GetByEmail retrieves a user from the database based on their email address.
func (m UserModel) GetByEmail(email string) (*User, error) {
	query := `
SELECT id, created_at, name, email, password_hash, activated, version
FROM users
WHERE email = $1`

	var user User
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Execute the query and scan the result into a user struct.
	err := m.DB.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Email,
		&user.Password.hash,
		&user.Activated,
		&user.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound // Return a specific error if no user is found.
		default:
			return nil, err // Return any other errors that occur.
		}
	}
	return &user, nil
}

// Update modifies an existing user's details in the database, using optimistic concurrency control.
func (m UserModel) Update(user *User) error {
	query := `
UPDATE users
SET name = $1, email = $2, password_hash = $3, activated = $4, version = version + 1
WHERE id = $5 AND version = $6
RETURNING version`

	args := []interface{}{
		user.Name,
		user.Email,
		user.Password.hash,
		user.Activated,
		user.ID,
		user.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Execute the query and scan the returned version into the user struct.
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail // Return a specific error if the email is already in use.
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict // Return a specific error if there is an edit conflict.
		default:
			return err // Return any other errors that occur.
		}
	}
	return nil
}

// GetForToken retrieves a user based on a token's hash, scope, and expiry.
func (m UserModel) GetForToken(tokenScope, tokenPlaintext string) (*User, error) {
	tokenHash := sha256.Sum256([]byte(tokenPlaintext)) // Hash the plaintext token using SHA-256.

	query := `
SELECT users.id, users.created_at, users.name, users.email, users.password_hash, users.activated, users.version
FROM users
INNER JOIN tokens
ON users.id = tokens.user_id
WHERE tokens.hash = $1
AND tokens.scope = $2
AND tokens.expiry > $3`

	args := []interface{}{tokenHash[:], tokenScope, time.Now()}
	var user User
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Execute the query and scan the result into a user struct.
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Email,
		&user.Password.hash,
		&user.Activated,
		&user.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound // Return a specific error if no user is found.
		default:
			return nil, err // Return any other errors that occur.
		}
	}

	return &user, nil
}

// Get retrieves a user from the database based on their ID.
func (m UserModel) Get(id int64) (*User, error) {
	query := `
SELECT id, created_at, name, email, password_hash, activated, version
FROM users
WHERE id = $1`

	var user User
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Execute the query and scan the result into a user struct.
	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Email,
		&user.Password.hash,
		&user.Activated,
		&user.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound // Return a specific error if no user is found.
		default:
			return nil, err // Return any other errors that occur.
		}
	}
	return &user, nil
}
