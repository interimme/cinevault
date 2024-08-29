package data

import (
	"cinevault.interimme.net/internal/validator"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"time"
)

// Movie represents a movie record in the database.
type Movie struct {
	ID        int64     `json:"id"`                // Unique identifier for the movie.
	CreatedAt time.Time `json:"-"`                 // Timestamp when the movie was created. This field is not included in the JSON response.
	Title     string    `json:"title"`             // The title of the movie.
	Year      int32     `json:"year,omitempty"`    // The release year of the movie. Omitted from JSON if not provided.
	Runtime   Runtime   `json:"runtime,omitempty"` // The runtime of the movie in minutes. Omitted from JSON if not provided.
	Genres    []string  `json:"genres,omitempty"`  // A list of genres the movie belongs to. Omitted from JSON if not provided.
	Version   int32     `json:"version"`           // The version number of the movie record for optimistic concurrency control.
}

// ValidateMovie validates the fields of a Movie struct to ensure they meet the required criteria.
func ValidateMovie(v *validator.Validator, movie *Movie) {
	v.Check(movie.Title != "", "title", "must be provided")
	v.Check(len(movie.Title) <= 500, "title", "must not be more than 500 bytes long")
	v.Check(movie.Year != 0, "year", "must be provided")
	v.Check(movie.Year >= 1888, "year", "must be greater than 1888") // The year 1888 is chosen because it's the year of the first known film.
	v.Check(movie.Year <= int32(time.Now().Year()), "year", "must not be in the future")
	v.Check(movie.Runtime != 0, "runtime", "must be provided")
	v.Check(movie.Runtime > 0, "runtime", "must be a positive integer")
	v.Check(movie.Genres != nil, "genres", "must be provided")
	v.Check(len(movie.Genres) >= 1, "genres", "must contain at least 1 genre")
	v.Check(len(movie.Genres) <= 5, "genres", "must not contain more than 5 genres")
	v.Check(validator.Unique(movie.Genres), "genres", "must not contain duplicate values")
}

// MovieModel represents the methods that can be performed on the movies in the database.
type MovieModel struct {
	DB *sql.DB // Database connection pool.
}

// Insert adds a new movie record to the database.
func (m MovieModel) Insert(movie *Movie) error {
	query := `
INSERT INTO movies (title, year, runtime, genres)
VALUES ($1, $2, $3, $4)
RETURNING id, created_at, version`
	args := []interface{}{movie.Title, movie.Year, movie.Runtime, pq.Array(movie.Genres)}
	// Create a context with a 3-second timeout for executing the query.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Execute the query and scan the returned id, created_at, and version into the movie struct.
	return m.DB.QueryRowContext(ctx, query, args...).Scan(&movie.ID, &movie.CreatedAt, &movie.Version)
}

// Get retrieves a specific movie record from the database by its ID.
func (m MovieModel) Get(id int64) (*Movie, error) {
	if id < 1 {
		return nil, ErrRecordNotFound // Return an error if the ID is invalid.
	}

	query := `
SELECT id, created_at, title, year, runtime, genres, version
FROM movies
WHERE id = $1`
	var movie Movie
	// Create a context with a 3-second timeout for executing the query.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Execute the query and scan the result into a movie struct.
	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&movie.ID,
		&movie.CreatedAt,
		&movie.Title,
		&movie.Year,
		&movie.Runtime,
		pq.Array(&movie.Genres),
		&movie.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound // Return a custom error if no rows are found.
		default:
			return nil, err // Return any other errors that occur.
		}
	}
	return &movie, nil
}

// Update modifies the details of an existing movie record in the database.
func (m MovieModel) Update(movie *Movie) error {
	query := `
UPDATE movies
SET title = $1, year = $2, runtime = $3, genres = $4, version = version + 1
WHERE id = $5 AND version = $6
RETURNING version`
	args := []interface{}{
		movie.Title,
		movie.Year,
		movie.Runtime,
		pq.Array(movie.Genres),
		movie.ID,
		movie.Version,
	}

	// Create a context with a 3-second timeout for executing the query.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Execute the update query and scan the returned version into the movie struct.
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&movie.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict // Return a custom error if there is an edit conflict.
		default:
			return err // Return any other errors that occur.
		}
	}
	return nil
}

// Delete removes a specific movie record from the database by its ID.
func (m MovieModel) Delete(id int64) error {
	if id < 1 {
		return ErrRecordNotFound // Return an error if the ID is invalid.
	}
	query := `
DELETE FROM movies
WHERE id = $1`

	// Create a context with a 3-second timeout for executing the query.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Execute the delete query.
	result, err := m.DB.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrRecordNotFound // Return a custom error if no rows are affected (i.e., the movie was not found).
	}
	return nil
}

// GetAll retrieves all movie records that match the provided title and genres, and applies pagination and sorting.
func (m MovieModel) GetAll(title string, genres []string, filters Filters) ([]*Movie, Metadata, error) {
	query := fmt.Sprintf(`
SELECT count(*) OVER(), id, created_at, title, year, runtime, genres, version
FROM movies
WHERE (to_tsvector('simple', title) @@ plainto_tsquery('simple', $1) OR $1 = '')
AND (genres @> $2 OR $2 = '{}')
ORDER BY %s %s, id ASC
LIMIT $3 OFFSET $4`, filters.sortColumn(), filters.sortDirection())

	// Create a context with a 3-second timeout for executing the query.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Prepare the arguments for the query.
	args := []interface{}{title, pq.Array(genres), filters.limit(), filters.offset()}
	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	movies := []*Movie{}
	// Loop through the result set and scan each row into a Movie struct.
	for rows.Next() {
		var movie Movie
		err := rows.Scan(
			&totalRecords,
			&movie.ID,
			&movie.CreatedAt,
			&movie.Title,
			&movie.Year,
			&movie.Runtime,
			pq.Array(&movie.Genres),
			&movie.Version,
		)
		if err != nil {
			return nil, Metadata{}, err
		}
		movies = append(movies, &movie) // Add each movie to the slice.
	}
	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	// Calculate pagination metadata for the result set.
	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return movies, metadata, nil
}
