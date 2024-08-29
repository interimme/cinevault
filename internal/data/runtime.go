package data

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidRuntimeFormat is an error that indicates the runtime format is invalid.
var ErrInvalidRuntimeFormat = errors.New("invalid runtime format")

// Runtime is a custom type that represents the runtime of a movie in minutes.
type Runtime int32

// MarshalJSON implements the json.Marshaler interface for the Runtime type.
// It converts the Runtime value to a JSON-encoded string in the format "<number> mins".
func (r Runtime) MarshalJSON() ([]byte, error) {
	// Convert the Runtime value to a string with the format "<number> mins".
	jsonValue := fmt.Sprintf("%d mins", r)

	// Quote the string to make it a valid JSON string.
	quotedJSONValue := strconv.Quote(jsonValue)

	// Return the quoted JSON string as a byte slice.
	return []byte(quotedJSONValue), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for the Runtime type.
// It parses a JSON-encoded string in the format "<number> mins" and converts it to a Runtime value.
func (r *Runtime) UnmarshalJSON(jsonValue []byte) error {
	// Remove the surrounding quotes from the JSON string value.
	unquotedJSONValue, err := strconv.Unquote(string(jsonValue))
	if err != nil {
		return ErrInvalidRuntimeFormat // Return an error if the string cannot be unquoted.
	}

	// Split the unquoted string into two parts: the number and the unit (e.g., "123 mins").
	parts := strings.Split(unquotedJSONValue, " ")

	// Check if the split result is exactly two parts and the unit is "mins".
	if len(parts) != 2 || parts[1] != "mins" {
		return ErrInvalidRuntimeFormat // Return an error if the format is incorrect.
	}

	// Parse the number part into an int32.
	i, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		return ErrInvalidRuntimeFormat // Return an error if the number cannot be parsed.
	}

	// Convert the parsed integer to a Runtime type and assign it to the receiver.
	*r = Runtime(i)
	return nil // Return nil to indicate successful parsing.
}
