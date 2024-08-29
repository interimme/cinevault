package data

import (
	"cinevault.interimme.net/internal/validator"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"time"
)

// Define constants for the different token scopes used in the application.
const (
	ScopeActivation     = "activation"     // Token scope for account activation.
	ScopeAuthentication = "authentication" // Token scope for user authentication.
	ScopePasswordReset  = "password-reset" // Token scope for password reset.
)

// Token struct represents a token with its plaintext value, hashed value, associated user ID, expiry time, and scope.
type Token struct {
	Plaintext string    `json:"token"`  // Plaintext representation of the token.
	Hash      []byte    `json:"-"`      // SHA-256 hash of the plaintext token (not included in JSON output).
	UserID    int64     `json:"-"`      // ID of the user to whom the token belongs (not included in JSON output).
	Expiry    time.Time `json:"expiry"` // Expiry time of the token.
	Scope     string    `json:"-"`      // Scope of the token (e.g., activation, authentication, password reset) (not included in JSON output).
}

// generateToken creates a new Token struct for a specific user, with a given time-to-live (TTL) and scope.
func generateToken(userID int64, ttl time.Duration, scope string) (*Token, error) {
	// Initialize a new Token struct with the provided user ID, expiry time, and scope.
	token := &Token{
		UserID: userID,
		Expiry: time.Now().Add(ttl), // Set the expiry time to the current time plus the TTL.
		Scope:  scope,
	}

	// Create a slice of 16 random bytes to use as the base for the token.
	randomBytes := make([]byte, 16)

	// Fill the slice with random bytes.
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err // Return an error if the random byte generation fails.
	}

	// Encode the random bytes to a base32 string without padding to create the plaintext token.
	token.Plaintext = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)

	// Generate a SHA-256 hash of the plaintext token and store it in the Hash field.
	hash := sha256.Sum256([]byte(token.Plaintext))
	token.Hash = hash[:]
	return token, nil // Return the generated token.
}

// ValidateTokenPlaintext validates that the provided token plaintext meets the expected criteria.
func ValidateTokenPlaintext(v *validator.Validator, tokenPlaintext string) {
	// Check that the token is not empty.
	v.Check(tokenPlaintext != "", "token", "must be provided")

	// Check that the token length is exactly 26 characters.
	v.Check(len(tokenPlaintext) == 26, "token", "must be 26 bytes long")
}

// TokenModel struct wraps a database connection pool and provides methods for working with tokens.
type TokenModel struct {
	DB *sql.DB
}

// New generates a new token for a user and inserts it into the database.
func (m TokenModel) New(userID int64, ttl time.Duration, scope string) (*Token, error) {
	// Generate a new token.
	token, err := generateToken(userID, ttl, scope)
	if err != nil {
		return nil, err
	}

	// Insert the token into the database.
	err = m.Insert(token)
	return token, err // Return the generated token and any error from the insert operation.
}

// Insert adds a new token record to the database.
func (m TokenModel) Insert(token *Token) error {
	// SQL query to insert a new token into the tokens table.
	query := `
INSERT INTO tokens (hash, user_id, expiry, scope)
VALUES ($1, $2, $3, $4)`

	// Arguments for the SQL query.
	args := []interface{}{token.Hash, token.UserID, token.Expiry, token.Scope}

	// Create a context with a 3-second timeout for executing the query.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Execute the query and insert the token into the database.
	_, err := m.DB.ExecContext(ctx, query, args...)
	return err // Return any error encountered during query execution.
}

// DeleteAllForUser deletes all tokens for a specific user and scope from the database.
func (m TokenModel) DeleteAllForUser(scope string, userID int64) error {
	// SQL query to delete all tokens for a specific user and scope.
	query := `
DELETE FROM tokens
WHERE scope = $1 AND user_id = $2`

	// Create a context with a 3-second timeout for executing the query.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Execute the query and delete the tokens from the database.
	_, err := m.DB.ExecContext(ctx, query, scope, userID)
	return err // Return any error encountered during query execution.
}
