package main

import (
	"cinevault.interimme.net/internal/data"
	"cinevault.interimme.net/internal/jsonlog"
	"cinevault.interimme.net/internal/mailer"
	"context"
	"database/sql"
	"expvar"
	"flag"
	"fmt"
	_ "github.com/lib/pq"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Global variables for build information
var (
	buildTime string
	version   string
)

// config struct holds all configuration settings for the application.
type config struct {
	port int      // Port for the API server
	env  string   // Environment (development, staging, production)
	db   struct { // Database configuration
		dsn          string // Data Source Name for PostgreSQL connection
		maxOpenConns int    // Maximum number of open connections to the database
		maxIdleConns int    // Maximum number of idle connections in the pool
		maxIdleTime  string // Maximum time a connection can remain idle
	}
	limiter struct { // Rate limiter settings
		enabled bool    // Enable rate limiter
		rps     float64 // Maximum requests per second
		burst   int     // Maximum burst size
	}
	smtp struct { // SMTP settings for sending emails
		host     string // SMTP host
		port     int    // SMTP port
		username string // SMTP username
		password string // SMTP password
		sender   string // SMTP sender email address
	}
	cors struct { // CORS settings
		trustedOrigins []string // Trusted origins for CORS
	}
	jwt struct { // JWT settings
		secret string // Secret key for signing JWTs
	}
}

// application struct holds all dependencies for the application, including configuration, logger, models, mailer, and wait group.
type application struct {
	config config          // Application configuration
	logger *jsonlog.Logger // Custom logger for structured JSON logging
	models data.Models     // Data models for interacting with the database
	mailer mailer.Mailer   // Mailer for sending emails
	wg     sync.WaitGroup  // Wait group for managing background goroutines
}

// main is the entry point for the application.
func main() {
	var cfg config

	// Command-line flags for configuration settings
	flag.IntVar(&cfg.port, "port", 4000, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")

	// Database connection settings
	flag.StringVar(&cfg.db.dsn, "db-dsn", "", "PostgreSQL DSN")
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.StringVar(&cfg.db.maxIdleTime, "db-max-idle-time", "15m", "PostgreSQL max connection idle time")

	// Rate limiter settings
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")
	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")

	// SMTP settings for sending emails
	flag.StringVar(&cfg.smtp.host, "smtp-host", "smtp.mailtrap.io", "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", 2525, "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", "8e3787e43c2023", "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", "f5539d047c69f7", "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", "Cinevault <no-reply@cinevault.interimme.net>", "SMTP sender")

	// CORS trusted origins setting
	flag.Func("cors-trusted-origins", "Trusted CORS origins (space separated)", func(val string) error {
		cfg.cors.trustedOrigins = strings.Fields(val)
		return nil
	})

	// JWT secret setting
	flag.StringVar(&cfg.jwt.secret, "jwt-secret", "", "JWT secret")

	// Display version flag
	displayVersion := flag.Bool("version", false, "Display version and exit")

	// Parse command-line flags
	flag.Parse()

	// Display version and exit if the version flag is set
	if *displayVersion {
		fmt.Printf("Version:\t%s\n", version)
		fmt.Printf("Build time:\t%s\n", buildTime)
		os.Exit(0)
	}

	// Initialize logger
	logger := jsonlog.New(os.Stdout, jsonlog.LevelInfo)

	// Open database connection
	db, err := openDB(cfg)
	if err != nil {
		logger.PrintFatal(err, nil)
	}
	defer db.Close() // Ensure the database connection pool is closed before the application exits

	logger.PrintInfo("database connection pool established", nil)

	// Publish application metrics using expvar
	expvar.NewString("version").Set(version)
	expvar.Publish("goroutines", expvar.Func(func() interface{} {
		return runtime.NumGoroutine()
	}))
	expvar.Publish("database", expvar.Func(func() interface{} {
		return db.Stats()
	}))
	expvar.Publish("timestamp", expvar.Func(func() interface{} {
		return time.Now().Unix()
	}))

	// Initialize the application struct with dependencies
	app := &application{
		config: cfg,
		logger: logger,
		models: data.NewModels(db),
		mailer: mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender),
	}

	// Start the server
	err = app.serve()
	if err != nil {
		logger.PrintFatal(err, nil)
	}
}

// openDB establishes a new database connection using the configuration settings and returns a sql.DB instance.
// It also verifies the connection is available by pinging the database.
func openDB(cfg config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.db.dsn) // Open a new database connection using the PostgreSQL driver
	if err != nil {
		return nil, err
	}

	// Set the maximum number of open connections
	db.SetMaxOpenConns(cfg.db.maxOpenConns)

	// Set the maximum number of idle connections
	db.SetMaxIdleConns(cfg.db.maxIdleConns)

	// Parse and set the maximum idle time for connections
	duration, err := time.ParseDuration(cfg.db.maxIdleTime)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxIdleTime(duration)

	// Create a context with a timeout to ensure the ping does not block indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Ping the database to verify the connection is established
	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	return db, nil
}
