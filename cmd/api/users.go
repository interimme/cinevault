package main

import (
	"cinevault.interimme.net/internal/data"
	"cinevault.interimme.net/internal/validator"
	"errors"
	"net/http"
	"time"
)

// registerUserHandler handles requests to register a new user.
func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	// Struct to hold the input data from the request body.
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	// Read the JSON request body into the input struct.
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Create a new user instance with the input data.
	user := &data.User{
		Name:      input.Name,
		Email:     input.Email,
		Activated: false, // New users start as not activated.
	}

	// Set the user's password.
	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Initialize a new validator instance.
	v := validator.New()

	// Validate the user's data.
	if data.ValidateUser(v, user); !v.Valid() {
		// If validation fails, respond with a 422 Unprocessable Entity error.
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Insert the new user into the database.
	err = app.models.Users.Insert(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateEmail):
			// If the email already exists, respond with a validation error.
			v.AddError("email", "a user with this email address already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			// Respond with a server error for other types of errors.
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Add default permissions for the new user.
	err = app.models.Permissions.AddForUser(user.ID, "movies:read")
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Generate an activation token for the new user.
	token, err := app.models.Tokens.New(user.ID, 3*24*time.Hour, data.ScopeActivation)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Send a welcome email with the activation token in the background.
	app.background(func() {
		data := map[string]interface{}{
			"activationToken": token.Plaintext,
			"userID":          user.ID,
		}
		err = app.mailer.Send(user.Email, "user_welcome.tmpl", data)
		if err != nil {
			app.logger.PrintError(err, nil)
		}
	})

	// Respond with a 202 Accepted status to indicate the registration was successful.
	err = app.writeJSON(w, http.StatusAccepted, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// activateUserHandler handles requests to activate a user account.
func (app *application) activateUserHandler(w http.ResponseWriter, r *http.Request) {
	// Struct to hold the input token from the request body.
	var input struct {
		TokenPlaintext string `json:"token"`
	}

	// Read the JSON request body into the input struct.
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Initialize a new validator instance.
	v := validator.New()

	// Validate the token plaintext.
	if data.ValidateTokenPlaintext(v, input.TokenPlaintext); !v.Valid() {
		// If validation fails, respond with a 422 Unprocessable Entity error.
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Retrieve the user associated with the activation token.
	user, err := app.models.Users.GetForToken(data.ScopeActivation, input.TokenPlaintext)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			// If no user is found, respond with a validation error.
			v.AddError("token", "invalid or expired activation token")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			// Respond with a server error for other types of errors.
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Activate the user account.
	user.Activated = true

	// Update the user's status in the database.
	err = app.models.Users.Update(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			// If there is an edit conflict, respond with a 409 Conflict error.
			app.editConflictResponse(w, r)
		default:
			// Respond with a server error for other types of errors.
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Delete all activation tokens for the user since they are now activated.
	err = app.models.Tokens.DeleteAllForUser(data.ScopeActivation, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Respond with a 200 OK status and the updated user data in JSON format.
	err = app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// updateUserPasswordHandler handles requests to update a user's password.
func (app *application) updateUserPasswordHandler(w http.ResponseWriter, r *http.Request) {
	// Struct to hold the input password and token from the request body.
	var input struct {
		Password       string `json:"password"`
		TokenPlaintext string `json:"token"`
	}

	// Read the JSON request body into the input struct.
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Initialize a new validator instance.
	v := validator.New()

	// Validate the password and token plaintext.
	data.ValidatePasswordPlaintext(v, input.Password)
	data.ValidateTokenPlaintext(v, input.TokenPlaintext)

	if !v.Valid() {
		// If validation fails, respond with a 422 Unprocessable Entity error.
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Retrieve the user associated with the password reset token.
	user, err := app.models.Users.GetForToken(data.ScopePasswordReset, input.TokenPlaintext)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			// If no user is found, respond with a validation error.
			v.AddError("token", "invalid or expired password reset token")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			// Respond with a server error for other types of errors.
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Update the user's password.
	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Update the user's data in the database.
	err = app.models.Users.Update(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			// If there is an edit conflict, respond with a 409 Conflict error.
			app.editConflictResponse(w, r)
		default:
			// Respond with a server error for other types of errors.
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Delete all password reset tokens for the user after a successful password reset.
	err = app.models.Tokens.DeleteAllForUser(data.ScopePasswordReset, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Respond with a confirmation message that the password was reset successfully.
	env := envelope{"message": "your password was successfully reset"}
	err = app.writeJSON(w, http.StatusOK, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
