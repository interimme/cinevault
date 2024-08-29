package data

import (
	"context"
	"database/sql"
	"github.com/lib/pq"
	"time"
)

// Permissions represents a slice of permission codes as strings.
type Permissions []string

// Include checks if a specific permission code exists within the Permissions slice.
func (p Permissions) Include(code string) bool {
	for i := range p {
		if code == p[i] {
			return true // Return true if the permission code is found.
		}
	}
	return false // Return false if the permission code is not found.
}

// PermissionModel represents the data access object for permissions-related operations.
type PermissionModel struct {
	DB *sql.DB // Database connection pool.
}

// GetAllForUser retrieves all permission codes for a specific user from the database.
func (m PermissionModel) GetAllForUser(userID int64) (Permissions, error) {
	// SQL query to select all permission codes associated with a specific user.
	query := `
SELECT permissions.code
FROM permissions
INNER JOIN users_permissions ON users_permissions.permission_id = permissions.id
INNER JOIN users ON users_permissions.user_id = users.id
WHERE users.id = $1`

	// Create a context with a 3-second timeout for executing the query.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Execute the query with the user ID as a parameter.
	rows, err := m.DB.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err // Return an error if the query fails.
	}
	defer rows.Close()

	var permissions Permissions
	// Iterate over the result set and append each permission to the permissions slice.
	for rows.Next() {
		var permission string
		err := rows.Scan(&permission)
		if err != nil {
			return nil, err // Return an error if scanning fails.
		}
		permissions = append(permissions, permission)
	}

	// Check for any errors encountered during iteration over the rows.
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return permissions, nil // Return the permissions slice.
}

// AddForUser adds new permissions for a specific user in the database.
func (m PermissionModel) AddForUser(userID int64, codes ...string) error {
	// SQL query to insert new user permissions.
	query := `
INSERT INTO users_permissions
SELECT $1, permissions.id FROM permissions WHERE permissions.code = ANY($2)`

	// Create a context with a 3-second timeout for executing the query.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Execute the query with the user ID and permission codes as parameters.
	_, err := m.DB.ExecContext(ctx, query, userID, pq.Array(codes))
	return err // Return any error encountered during query execution.
}
