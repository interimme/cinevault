package main

import (
	"cinevault.interimme.net/internal/data"
	"cinevault.interimme.net/internal/validator"
	"errors"
	"github.com/pascaldekloe/jwt"
	"net/http"
	"strconv"
	"time"
)

// createAuthenticationTokenHandler handles requests to generate a new authentication token.
func (app *application) createAuthenticationTokenHandler(w http.ResponseWriter, r *http.Request) {
	// Struct to hold the input email and password from the request.
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	// Read JSON request body into the input struct.
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Initialize a new validator instance.
	v := validator.New()

	// Validate email and password fields.
	data.ValidateEmail(v, input.Email)
	data.ValidatePasswordPlaintext(v, input.Password)

	if !v.Valid() {
		// Respond with validation errors if input is invalid.
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Retrieve the user by email.
	user, err := app.models.Users.GetByEmail(input.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			// Respond with an invalid credentials error if no user is found.
			app.invalidCredentialsResponse(w, r)
		default:
			// Respond with a server error for other types of errors.
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Check if the provided password matches the stored password.
	match, err := user.Password.Matches(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if !match {
		// Respond with an invalid credentials error if passwords do not match.
		app.invalidCredentialsResponse(w, r)
		return
	}

	// Define JWT claims.
	var claims jwt.Claims
	claims.Subject = strconv.FormatInt(user.ID, 10)
	claims.Issued = jwt.NewNumericTime(time.Now())
	claims.NotBefore = jwt.NewNumericTime(time.Now())
	claims.Expires = jwt.NewNumericTime(time.Now().Add(24 * time.Hour))
	claims.Issuer = "cinevault.interimme.net"
	claims.Audiences = []string{"cinevault.interimme.net"}

	// Sign the JWT claims using HMAC SHA-256.
	jwtBytes, err := claims.HMACSign(jwt.HS256, []byte(app.config.jwt.secret))
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Respond with the generated JWT.
	err = app.writeJSON(w, http.StatusCreated, envelope{"authentication_token": string(jwtBytes)}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// createPasswordResetTokenHandler handles requests to generate a password reset token.
func (app *application) createPasswordResetTokenHandler(w http.ResponseWriter, r *http.Request) {
	// Struct to hold the input email from the request.
	var input struct {
		Email string `json:"email"`
	}

	// Read JSON request body into the input struct.
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Initialize a new validator instance.
	v := validator.New()

	// Validate email field.
	if data.ValidateEmail(v, input.Email); !v.Valid() {
		// Respond with validation errors if the email is invalid.
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Retrieve the user by email.
	user, err := app.models.Users.GetByEmail(input.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			// Respond with validation error if no user is found.
			v.AddError("email", "no matching email address found")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			// Respond with a server error for other types of errors.
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Check if the user account is activated.
	if !user.Activated {
		v.AddError("email", "user account must be activated")
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Generate a new password reset token for the user.
	token, err := app.models.Tokens.New(user.ID, 45*time.Minute, data.ScopePasswordReset)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Send password reset email in the background.
	app.background(func() {
		data := map[string]interface{}{
			"passwordResetToken": token.Plaintext,
		}

		err = app.mailer.Send(user.Email, "token_password_reset.tmpl", data)
		if err != nil {
			app.logger.PrintError(err, nil)
		}
	})

	// Respond with a message indicating that password reset instructions will be sent.
	env := envelope{"message": "an email will be sent to you containing password reset instructions"}
	err = app.writeJSON(w, http.StatusAccepted, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// createActivationTokenHandler handles requests to generate an activation token for a user account.
func (app *application) createActivationTokenHandler(w http.ResponseWriter, r *http.Request) {
	// Struct to hold the input email from the request.
	var input struct {
		Email string `json:"email"`
	}

	// Read JSON request body into the input struct.
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Initialize a new validator instance.
	v := validator.New()

	// Validate email field.
	if data.ValidateEmail(v, input.Email); !v.Valid() {
		// Respond with validation errors if the email is invalid.
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Retrieve the user by email.
	user, err := app.models.Users.GetByEmail(input.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			// Respond with validation error if no user is found.
			v.AddError("email", "no matching email address found")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			// Respond with a server error for other types of errors.
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Check if the user has already been activated.
	if user.Activated {
		v.AddError("email", "user has already been activated")
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Generate a new activation token for the user.
	token, err := app.models.Tokens.New(user.ID, 3*24*time.Hour, data.ScopeActivation)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Send activation email in the background.
	app.background(func() {
		data := map[string]interface{}{
			"activationToken": token.Plaintext,
		}

		err = app.mailer.Send(user.Email, "token_activation.tmpl", data)
		if err != nil {
			app.logger.PrintError(err, nil)
		}
	})

	// Respond with a message indicating that activation instructions will be sent.
	env := envelope{"message": "an email will be sent to you containing activation instructions"}
	err = app.writeJSON(w, http.StatusAccepted, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
