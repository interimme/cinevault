package main

import (
	"cinevault.interimme.net/internal/data"
	"cinevault.interimme.net/internal/validator"
	"errors"
	"expvar"
	"fmt"
	"github.com/felixge/httpsnoop"
	"github.com/tomasen/realip"
	"golang.org/x/time/rate"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// recoverPanic is a middleware that recovers from any panic that occurs during the HTTP request handling.
// It logs the panic and returns a 500 Internal Server Error response to the client.
func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Set the Connection header to close to prevent the client from reusing the connection.
				w.Header().Set("Connection", "close")
				// Log the error and send a generic server error response.
				app.serverErrorResponse(w, r, fmt.Errorf("%s", err))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// rateLimit is a middleware that implements rate limiting for incoming HTTP requests based on the client's IP address.
// It uses a token bucket algorithm to control the rate of requests.
func (app *application) rateLimit(next http.Handler) http.Handler {
	type client struct {
		limiter  *rate.Limiter // Rate limiter for the client
		lastSeen time.Time     // Timestamp of the last request from the client
	}

	var (
		mu      sync.Mutex                 // Mutex to protect the clients map
		clients = make(map[string]*client) // Map to store rate limiter instances per client IP
	)

	// Background goroutine to periodically clean up old clients from the map.
	go func() {
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for ip, client := range clients {
				// Remove clients that haven't been seen in the last 3 minutes.
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if app.config.limiter.enabled {
			// Extract the client's IP address from the request.
			ip := realip.FromRequest(r)
			mu.Lock()
			// Initialize a new rate limiter for the client if it doesn't exist.
			if _, found := clients[ip]; !found {
				clients[ip] = &client{
					limiter: rate.NewLimiter(rate.Limit(app.config.limiter.rps), app.config.limiter.burst),
				}
			}
			clients[ip].lastSeen = time.Now()
			// Check if the client is allowed to make a request.
			if !clients[ip].limiter.Allow() {
				mu.Unlock()
				app.rateLimitExceededResponse(w, r)
				return
			}
			mu.Unlock()
		}
		next.ServeHTTP(w, r)
	})
}

// authenticate is a middleware that checks for a valid authentication token in the request headers.
// If a valid token is found, the corresponding user is loaded into the request context.
func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set Vary header to ensure clients cache different responses based on the Authorization header.
		w.Header().Set("Vary", "Authorization")

		// Retrieve the Authorization header from the request.
		authorizationHeader := r.Header.Get("Authorization")

		if authorizationHeader == "" {
			// No Authorization header, proceed with an anonymous user.
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		// Split the header into its components.
		headerParts := strings.Split(authorizationHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			// Invalid Authorization header format.
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		token := headerParts[1]

		v := validator.New()

		// Validate the token format.
		if data.ValidateTokenPlaintext(v, token); !v.Valid() {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		// Fetch the user associated with the token from the database.
		user, err := app.models.Users.GetForToken(data.ScopeAuthentication, token)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrRecordNotFound):
				// Invalid token.
				app.invalidAuthenticationTokenResponse(w, r)
			default:
				// Server error.
				app.serverErrorResponse(w, r, err)
			}
			return
		}

		// Add the authenticated user to the request context.
		r = app.contextSetUser(r, user)

		next.ServeHTTP(w, r)
	})
}

// requireAuthenticatedUser is a middleware that ensures the user is authenticated before allowing access to the next handler.
func (app *application) requireAuthenticatedUser(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Retrieve the user from the request context.
		user := app.contextGetUser(r)
		if user.IsAnonymous() {
			// User is not authenticated.
			app.authenticationRequiredResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireActivatedUser is a middleware that ensures the user is both authenticated and has an activated account before allowing access.
func (app *application) requireActivatedUser(next http.HandlerFunc) http.HandlerFunc {
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Retrieve the user from the request context.
		user := app.contextGetUser(r)
		if !user.Activated {
			// User account is not activated.
			app.inactiveAccountResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})

	return app.requireAuthenticatedUser(fn)
}

// requirePermission is a middleware that checks if the user has the necessary permission to access the next handler.
func (app *application) requirePermission(code string, next http.HandlerFunc) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		// Retrieve the user from the request context.
		user := app.contextGetUser(r)
		// Fetch all permissions for the user from the database.
		permissions, err := app.models.Permissions.GetAllForUser(user.ID)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
		// Check if the user has the required permission.
		if !permissions.Include(code) {
			// User does not have the required permission.
			app.notPermittedResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	}
	return app.requireActivatedUser(fn)
}

// enableCORS is a middleware that adds the necessary headers to support Cross-Origin Resource Sharing (CORS).
func (app *application) enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add Vary headers to ensure clients cache different responses based on Origin and Access-Control-Request-Method headers.
		w.Header().Add("Vary", "Origin")
		w.Header().Add("Vary", "Access-Control-Request-Method")
		origin := r.Header.Get("Origin")
		if origin != "" {
			// Check if the request origin is in the list of trusted origins.
			for i := range app.config.cors.trustedOrigins {
				if origin == app.config.cors.trustedOrigins[i] {
					// Set the Access-Control-Allow-Origin header to allow the origin.
					w.Header().Set("Access-Control-Allow-Origin", origin)
					// Handle preflight requests.
					if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
						w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, PUT, PATCH, DELETE")
						w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
						w.WriteHeader(http.StatusOK)
						return
					}
					break
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// metrics is a middleware that tracks application metrics such as total requests received, total responses sent,
// and the processing time for each request.
func (app *application) metrics(next http.Handler) http.Handler {
	// Define expvar variables to hold the metrics.
	totalRequestsReceived := expvar.NewInt("total_requests_received")
	totalResponsesSent := expvar.NewInt("total_responses_sent")
	totalProcessingTimeMicroseconds := expvar.NewInt("total_processing_time_μs")
	totalResponsesSentByStatus := expvar.NewMap("total_responses_sent_by_status")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Increment the total number of requests received.
		totalRequestsReceived.Add(1)

		// Capture the metrics for the request.
		metrics := httpsnoop.CaptureMetrics(next, w, r)

		// Increment the total number of responses sent.
		totalResponsesSent.Add(1)
		// Add the processing time for the request to the total processing

		totalProcessingTimeMicroseconds.Add(metrics.Duration.Microseconds())
		totalResponsesSentByStatus.Add(strconv.Itoa(metrics.Code), 1)
	})
}
