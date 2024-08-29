package validator

import (
	"regexp"
)

// EmailRX is a regular expression pattern to validate the format of email addresses.
// It checks that the email conforms to the standard format with allowed characters and structure.
var (
	EmailRX = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
)

// Validator struct holds a map of validation errors, where the key is the field name and the value is the error message.
type Validator struct {
	Errors map[string]string // Maps field names to their corresponding error messages.
}

// New initializes a new Validator instance with an empty map for errors.
func New() *Validator {
	return &Validator{Errors: make(map[string]string)}
}

// Valid returns true if the Validator contains no errors.
func (v *Validator) Valid() bool {
	return len(v.Errors) == 0
}

// AddError adds an error message for a given field to the Validator, if an error does not already exist for that field.
func (v *Validator) AddError(key, message string) {
	if _, exists := v.Errors[key]; !exists {
		v.Errors[key] = message // Add the error message to the map if it doesn't already exist.
	}
}

// Check adds an error message to the Validator if the provided condition is false.
func (v *Validator) Check(ok bool, key, message string) {
	if !ok {
		v.AddError(key, message) // Add an error if the condition is not met.
	}
}

// In checks if a value is in a list of strings.
// It returns true if the value is found in the list.
func In(value string, list ...string) bool {
	for i := range list {
		if value == list[i] {
			return true
		}
	}
	return false // Return false if the value is not found in the list.
}

// Matches checks if a value matches a regular expression pattern.
// It returns true if the value matches the regex.
func Matches(value string, rx *regexp.Regexp) bool {
	return rx.MatchString(value)
}

// Unique checks if all values in a slice of strings are unique.
// It returns true if all values are unique.
func Unique(values []string) bool {
	uniqueValues := make(map[string]bool)
	for _, value := range values {
		uniqueValues[value] = true // Add the value to the map if it doesn't already exist.
	}
	return len(values) == len(uniqueValues) // Return true if all values are unique (no duplicates).
}
