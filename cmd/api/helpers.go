package main

import (
	"cinevault.interimme.net/internal/validator"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// readIDParam extracts the "id" parameter from the URL and converts it to an int64.
// Returns an error if the "id" parameter is missing, invalid, or less than 1.
func (app *application) readIDParam(r *http.Request) (int64, error) {
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid id parameter")
	}
	return id, nil
}

// envelope is a type alias for a map that holds JSON response data.
type envelope map[string]interface{}

// writeJSON writes a JSON response to the client with a specified status code and optional headers.
func (app *application) writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	// Marshal the data into a pretty-printed JSON format.
	js, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}
	js = append(js, '\n')

	// Add any additional headers to the response.
	for key, value := range headers {
		w.Header()[key] = value
	}

	// Set the Content-Type header to indicate JSON response.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status) // Write the HTTP status code to the response.
	w.Write(js)           // Write the JSON data to the response body.
	return nil
}

// readJSON reads and parses JSON data from the request body into the destination struct.
// Validates the JSON format and checks for various errors, such as syntax errors and unexpected fields.
func (app *application) readJSON(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	// Limit the size of the request body to prevent large payloads from causing issues.
	maxBytes := 1_048_576
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // Disallow unknown fields to enforce strict schema validation.

	// Decode JSON data into the destination struct.
	err := dec.Decode(dst)

	// Handle various JSON parsing errors.
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError

		switch {
		case errors.As(err, &syntaxError):
			// JSON syntax error.
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			// Unexpected end of input.
			return errors.New("body contains badly-formed JSON")
		case errors.As(err, &unmarshalTypeError):
			// Incorrect type error.
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)
		case errors.Is(err, io.EOF):
			// Empty body error.
			return errors.New("body must not be empty")
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			// Unknown field error.
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unknown key %s", fieldName)
		case err.Error() == "http: request body too large":
			// Request body too large error.
			return fmt.Errorf("body must not be larger than %d bytes", maxBytes)
		case errors.As(err, &invalidUnmarshalError):
			// Invalid unmarshal error. Should never happen; panic to alert developers.
			panic(err)
		default:
			// Other errors.
			return err
		}
	}

	// Ensure the JSON data only contains a single value.
	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must only contain a single JSON value")
	}
	return nil
}

// readString reads a string query parameter from the URL query string. If the parameter is missing, returns a default value.
func (app *application) readString(qs url.Values, key string, defaultValue string) string {
	s := qs.Get(key)
	if s == "" {
		return defaultValue
	}
	return s
}

// readCSV reads a CSV (comma-separated values) query parameter from the URL query string and returns it as a slice of strings.
// If the parameter is missing, returns a default value.
func (app *application) readCSV(qs url.Values, key string, defaultValue []string) []string {
	csv := qs.Get(key)

	if csv == "" {
		return defaultValue
	}

	return strings.Split(csv, ",")
}

// readInt reads an integer query parameter from the URL query string and returns it as an int.
// If the parameter is missing or invalid, returns a default value and adds a validation error.
func (app *application) readInt(qs url.Values, key string, defaultValue int, v *validator.Validator) int {
	s := qs.Get(key)

	if s == "" {
		return defaultValue
	}

	i, err := strconv.Atoi(s)
	if err != nil {
		v.AddError(key, "must be an integer value")
		return defaultValue
	}

	return i
}

// background runs a function in a separate goroutine and recovers from any panic that occurs in the goroutine.
// This is useful for running background tasks without crashing the server if a panic occurs.
func (app *application) background(fn func()) {
	app.wg.Add(1) // Increment the wait group counter.

	go func() {
		defer app.wg.Done() // Decrement the wait group counter when the goroutine completes.

		defer func() {
			if err := recover(); err != nil {
				app.logger.PrintError(fmt.Errorf("%s", err), nil) // Log any panic that occurs.
			}
		}()

		fn() // Run the background function.
	}()
}
