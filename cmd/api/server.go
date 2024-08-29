package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// serve starts the HTTP server and manages graceful shutdowns.
func (app *application) serve() error {
	// Configure the HTTP server with settings from the application configuration.
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.port), // Server address, based on configured port.
		Handler:      app.routes(),                        // Set the handler to the routes defined in the application.
		IdleTimeout:  time.Minute,                         // Maximum time to keep idle connections alive.
		ReadTimeout:  10 * time.Second,                    // Maximum duration for reading the entire request.
		WriteTimeout: 30 * time.Second,                    // Maximum duration before timing out writes of the response.
	}

	// Channel to receive errors during server shutdown.
	shutdownError := make(chan error)

	// Goroutine to handle graceful server shutdown when an interrupt signal is received.
	go func() {
		// Channel to receive OS signals.
		quit := make(chan os.Signal, 1)
		// Notify the channel on SIGINT (Ctrl+C) or SIGTERM (termination) signals.
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		// Block until a signal is received.
		s := <-quit

		// Log the signal received.
		app.logger.PrintInfo("caught signal", map[string]string{
			"signal": s.String(),
		})

		// Create a context with a timeout for the shutdown process.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel() // Ensure the cancel function is called to free resources.

		// Attempt to gracefully shutdown the server.
		err := srv.Shutdown(ctx)
		if err != nil {
			// If there is an error during shutdown, send it to the shutdownError channel.
			shutdownError <- err
		}

		// Log message indicating the server is completing background tasks.
		app.logger.PrintInfo("completing background tasks", map[string]string{
			"addr": srv.Addr,
		})

		// Wait for any background goroutines to finish.
		app.wg.Wait()

		// Indicate that shutdown has completed without errors.
		shutdownError <- nil
	}()

	// Log message indicating the server is starting.
	app.logger.PrintInfo("starting server", map[string]string{
		"addr": srv.Addr,
		"env":  app.config.env,
	})

	// Start the HTTP server.
	err := srv.ListenAndServe()
	// If the error is not http.ErrServerClosed (which indicates a graceful shutdown), return the error.
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// Wait for the shutdown goroutine to send a completion or error message.
	err = <-shutdownError
	if err != nil {
		return err
	}

	// Log message indicating the server has stopped.
	app.logger.PrintInfo("stopped server", map[string]string{
		"addr": srv.Addr,
	})

	return nil
}
