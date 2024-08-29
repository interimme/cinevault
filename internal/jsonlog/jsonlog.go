package jsonlog

import (
	"encoding/json"
	"io"
	"os"
	"runtime/debug"
	"sync"
	"time"
)

// Level is a custom type for defining log levels.
type Level int8

// Log level constants to define different levels of logging severity.
const (
	LevelInfo  Level = iota // Info level logs, typically used for general informational messages. Value is 0.
	LevelError              // Error level logs, used for non-critical errors. Value is 1.
	LevelFatal              // Fatal level logs, used for critical errors after which the application cannot continue. Value is 2.
	LevelOff                // No logging. Value is 3.
)

// String converts the log level to its string representation.
func (l Level) String() string {
	switch l {
	case LevelInfo:
		return "INFO"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return ""
	}
}

// Logger struct defines a custom logger that writes logs to an output and filters messages below a certain severity level.
type Logger struct {
	out      io.Writer  // Destination for the log messages, such as os.Stdout or a file.
	minLevel Level      // Minimum log level to output messages for.
	mu       sync.Mutex // Mutex to synchronize log writes and prevent race conditions.
}

// New creates a new Logger instance.
func New(out io.Writer, minLevel Level) *Logger {
	return &Logger{
		out:      out,
		minLevel: minLevel,
	}
}

// PrintInfo logs a message at the INFO level.
func (l *Logger) PrintInfo(message string, properties map[string]string) {
	l.print(LevelInfo, message, properties)
}

// PrintError logs an error message at the ERROR level.
func (l *Logger) PrintError(err error, properties map[string]string) {
	l.print(LevelError, err.Error(), properties)
}

// PrintFatal logs an error message at the FATAL level and then exits the application.
func (l *Logger) PrintFatal(err error, properties map[string]string) {
	l.print(LevelFatal, err.Error(), properties)
	os.Exit(1) // Exit the application with a status code of 1 after logging a fatal error.
}

// print writes a log entry if the log level is greater than or equal to the minimum level.
func (l *Logger) print(level Level, message string, properties map[string]string) (int, error) {
	// Return immediately if the log level is below the minimum threshold.
	if level < l.minLevel {
		return 0, nil
	}

	// Define a struct to hold the log entry data.
	aux := struct {
		Level      string            `json:"level"`                // The log level (e.g., INFO, ERROR).
		Time       string            `json:"time"`                 // The current time in UTC format.
		Message    string            `json:"message"`              // The log message.
		Properties map[string]string `json:"properties,omitempty"` // Optional properties to include with the log message.
		Trace      string            `json:"trace,omitempty"`      // Stack trace, included for error levels and above.
	}{
		Level:      level.String(),
		Time:       time.Now().UTC().Format(time.RFC3339),
		Message:    message,
		Properties: properties,
	}

	// Include a stack trace if the log level is ERROR or higher.
	if level >= LevelError {
		aux.Trace = string(debug.Stack())
	}

	// Marshal the log entry to JSON.
	line, err := json.Marshal(aux)
	if err != nil {
		// If JSON marshaling fails, log the error in plain text.
		line = []byte(LevelError.String() + ": unable to marshal log message: " + err.Error())
	}

	// Ensure that log writes are atomic by locking the mutex.
	l.mu.Lock()
	defer l.mu.Unlock()

	// Write the log entry to the output, appending a newline.
	return l.out.Write(append(line, '\n'))
}

// Write logs a message at the ERROR level using the standard logger interface.
func (l *Logger) Write(message []byte) (n int, err error) {
	return l.print(LevelError, string(message), nil)
}
