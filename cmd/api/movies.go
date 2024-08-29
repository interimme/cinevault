package main

import (
	"cinevault.interimme.net/internal/data"
	"cinevault.interimme.net/internal/validator"
	"errors"
	"fmt"
	"net/http"
)

// createMovieHandler handles requests to create a new movie record.
func (app *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
	// Define a struct to hold the input data from the request body.
	var input struct {
		Title   string       `json:"title"`
		Year    int32        `json:"year"`
		Runtime data.Runtime `json:"runtime"`
		Genres  []string     `json:"genres"`
	}

	// Parse the JSON request body into the input struct.
	err := app.readJSON(w, r, &input)
	if err != nil {
		// If there's an error, respond with a 400 Bad Request error.
		app.badRequestResponse(w, r, err)
		return
	}

	// Create a new Movie struct using the input data.
	movie := &data.Movie{
		Title:   input.Title,
		Year:    input.Year,
		Runtime: input.Runtime,
		Genres:  input.Genres,
	}

	// Initialize a new validator instance.
	v := validator.New()

	// Validate the movie data.
	if data.ValidateMovie(v, movie); !v.Valid() {
		// If validation fails, respond with a 422 Unprocessable Entity error.
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Insert the movie record into the database.
	err = app.models.Movies.Insert(movie)
	if err != nil {
		// If there's a server error, respond with a 500 Internal Server Error.
		app.serverErrorResponse(w, r, err)
		return
	}

	// Set the Location header for the new movie resource.
	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/movies/%d", movie.ID))

	// Respond with a 201 Created status and the movie data in JSON format.
	err = app.writeJSON(w, http.StatusCreated, envelope{"movie": movie}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// showMovieHandler handles requests to retrieve a specific movie by ID.
func (app *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the movie ID from the URL parameters.
	id, err := app.readIDParam(r)
	if err != nil {
		// If the ID is invalid, respond with a 404 Not Found error.
		app.notFoundResponse(w, r)
		return
	}

	// Retrieve the movie from the database.
	movie, err := app.models.Movies.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			// If the movie is not found, respond with a 404 Not Found error.
			app.notFoundResponse(w, r)
		default:
			// For any other errors, respond with a 500 Internal Server Error.
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Respond with a 200 OK status and the movie data in JSON format.
	err = app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// updateMovieHandler handles requests to update an existing movie record.
func (app *application) updateMovieHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the movie ID from the URL parameters.
	id, err := app.readIDParam(r)
	if err != nil {
		// If the ID is invalid, respond with a 404 Not Found error.
		app.notFoundResponse(w, r)
		return
	}

	// Retrieve the existing movie from the database.
	movie, err := app.models.Movies.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			// If the movie is not found, respond with a 404 Not Found error.
			app.notFoundResponse(w, r)
		default:
			// For any other errors, respond with a 500 Internal Server Error.
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Define a struct to hold the input data from the request body.
	var input struct {
		Title   *string       `json:"title"`
		Year    *int32        `json:"year"`
		Runtime *data.Runtime `json:"runtime"`
		Genres  []string      `json:"genres"`
	}

	// Parse the JSON request body into the input struct.
	err = app.readJSON(w, r, &input)
	if err != nil {
		// If there's an error, respond with a 400 Bad Request error.
		app.badRequestResponse(w, r, err)
		return
	}

	// Update the movie fields if the input data is provided.
	if input.Title != nil {
		movie.Title = *input.Title
	}
	if input.Year != nil {
		movie.Year = *input.Year
	}
	if input.Runtime != nil {
		movie.Runtime = *input.Runtime
	}
	if input.Genres != nil {
		movie.Genres = input.Genres
	}

	// Initialize a new validator instance.
	v := validator.New()

	// Validate the updated movie data.
	if data.ValidateMovie(v, movie); !v.Valid() {
		// If validation fails, respond with a 422 Unprocessable Entity error.
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Update the movie record in the database.
	err = app.models.Movies.Update(movie)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			// If there is an edit conflict, respond with a 409 Conflict error.
			app.editConflictResponse(w, r)
		default:
			// For any other errors, respond with a 500 Internal Server Error.
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Respond with a 200 OK status and the updated movie data in JSON format.
	err = app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// deleteMovieHandler handles requests to delete a specific movie by ID.
func (app *application) deleteMovieHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the movie ID from the URL parameters.
	id, err := app.readIDParam(r)
	if err != nil {
		// If the ID is invalid, respond with a 404 Not Found error.
		app.notFoundResponse(w, r)
		return
	}

	// Delete the movie from the database.
	err = app.models.Movies.Delete(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			// If the movie is not found, respond with a 404 Not Found error.
			app.notFoundResponse(w, r)
		default:
			// For any other errors, respond with a 500 Internal Server Error.
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Respond with a 200 OK status and a message indicating successful deletion.
	err = app.writeJSON(w, http.StatusOK, envelope{"message": "movie successfully deleted"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// listMoviesHandler handles requests to list all movies with optional filtering, sorting, and pagination.
func (app *application) listMoviesHandler(w http.ResponseWriter, r *http.Request) {
	// Define a struct to hold the input data from the URL query string.
	var input struct {
		Title  string
		Genres []string
		data.Filters
	}

	// Initialize a new validator instance.
	v := validator.New()
	qs := r.URL.Query()

	// Read query parameters for filtering and pagination.
	input.Title = app.readString(qs, "title", "")
	input.Genres = app.readCSV(qs, "genres", []string{})
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)
	input.Filters.Sort = app.readString(qs, "sort", "id")
	input.Filters.SortSafelist = []string{"id", "title", "year", "runtime", "-id", "-title", "-year", "-runtime"}

	// Validate the filters.
	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		// If validation fails, respond with a 422 Unprocessable Entity error.
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Retrieve the list of movies from the database using the filters.
	movies, metadata, err := app.models.Movies.GetAll(input.Title, input.Genres, input.Filters)
	if err != nil {
		// For any server error, respond with a 500 Internal Server Error.
		app.serverErrorResponse(w, r, err)
		return
	}

	// Respond with a 200 OK status and the list of movies along with metadata in JSON format.
	err = app.writeJSON(w, http.StatusOK, envelope{"movies": movies, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
