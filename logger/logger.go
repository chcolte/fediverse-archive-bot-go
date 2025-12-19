package logger

import (
	"fmt"
	"log"
	"os"
	"sync"
)

type LogLevel int

const (
	// LevelError for error messages (always shown)
	LevelError LogLevel = iota
	// LevelInfo for general information (always shown)
	LevelInfo
	// LevelDebug for detailed debug information (shown only in verbose mode)
	LevelDebug
)

// Logger is the main logger struct
type Logger struct {
	verbose     bool
	mu          sync.Mutex
	errorLogger *log.Logger
	infoLogger  *log.Logger
	debugLogger *log.Logger
}

// Global logger instance
var defaultLogger *Logger

func init() {
	defaultLogger = New(false)
}

// New creates a new Logger instance
func New(verbose bool) *Logger {
	return &Logger{
		verbose:     verbose,
		errorLogger: log.New(os.Stderr, "[ERROR] ", log.LstdFlags),
		infoLogger:  log.New(os.Stdout, "[INFO]  ", log.LstdFlags),
		debugLogger: log.New(os.Stdout, "[DEBUG] ", log.LstdFlags),
	}
}

// SetVerbose sets the verbose mode for the default logger
func SetVerbose(verbose bool) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	defaultLogger.verbose = verbose
}

// IsVerbose returns whether verbose mode is enabled
func IsVerbose() bool {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	return defaultLogger.verbose
}

// SetFlags sets the output flags for all loggers (like log.SetFlags)
func SetFlags(flag int) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	defaultLogger.errorLogger.SetFlags(flag)
	defaultLogger.infoLogger.SetFlags(flag)
	defaultLogger.debugLogger.SetFlags(flag)
}

// Error logs an error message (always shown)
func Error(v ...interface{}) {
	defaultLogger.errorLogger.Output(2, fmt.Sprint(v...))
}

// Errorf logs a formatted error message (always shown)
func Errorf(format string, v ...interface{}) {
	defaultLogger.errorLogger.Output(2, fmt.Sprintf(format, v...))
}

// Info logs an informational message (always shown)
func Info(v ...interface{}) {
	defaultLogger.infoLogger.Output(2, fmt.Sprint(v...))
}

// Infof logs a formatted informational message (always shown)
func Infof(format string, v ...interface{}) {
	defaultLogger.infoLogger.Output(2, fmt.Sprintf(format, v...))
}

// Debug logs a debug message (shown only in verbose mode)
func Debug(v ...interface{}) {
	if defaultLogger.verbose {
		defaultLogger.debugLogger.Output(2, fmt.Sprint(v...))
	}
}

// Debugf logs a formatted debug message (shown only in verbose mode)
func Debugf(format string, v ...interface{}) {
	if defaultLogger.verbose {
		defaultLogger.debugLogger.Output(2, fmt.Sprintf(format, v...))
	}
}

// Fatal logs an error message and exits the program
func Fatal(v ...interface{}) {
	defaultLogger.errorLogger.Output(2, fmt.Sprint(v...))
	os.Exit(1)
}

// Fatalf logs a formatted error message and exits the program
func Fatalf(format string, v ...interface{}) {
	defaultLogger.errorLogger.Output(2, fmt.Sprintf(format, v...))
	os.Exit(1)
}

// Println is an alias for Info (for easier migration from standard log package)
func Println(v ...interface{}) {
	defaultLogger.infoLogger.Output(2, fmt.Sprint(v...))
}

// Printf is an alias for Infof (for easier migration from standard log package)
func Printf(format string, v ...interface{}) {
	defaultLogger.infoLogger.Output(2, fmt.Sprintf(format, v...))
}
