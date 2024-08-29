package data

import (
	"database/sql"
	"errors"
)

// Define common error messages for use throughout the data package.
var (
	ErrRecordNotFound = errors.New("record not found") // Error when a requested record does not exist in the database.
	ErrEditConflict   = errors.New("edit conflict")    // Error when a concurrent edit causes a conflict.
)

// Models struct is a container for different models (Movie, Permission, Token, User).
// This struct provides an easy way to access all the database models in one place.
type Models struct {
	Movies      MovieModel      // MovieModel handles operations related to the movies.
	Permissions PermissionModel // PermissionModel handles user permissions.
	Tokens      TokenModel      // TokenModel handles user tokens (e.g., for authentication).
	Users       UserModel       // UserModel handles user-related operations.
}

// NewModels initializes and returns a Models struct with a database connection pool.
// It is used to create instances of each model type with a shared database connection.
func NewModels(db *sql.DB) Models {
	return Models{
		Movies:      MovieModel{DB: db},      // Initialize MovieModel with the provided DB connection.
		Permissions: PermissionModel{DB: db}, // Initialize PermissionModel with the provided DB connection.
		Tokens:      TokenModel{DB: db},      // Initialize TokenModel with the provided DB connection.
		Users:       UserModel{DB: db},       // Initialize UserModel with the provided DB connection.
	}
}
