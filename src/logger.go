package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
)

type Severity int8

const (
	SeverityDebug Severity = iota
	SeverityInfo
	SeverityWarn
	SeverityError
	SeverityFatal
)

// ANSI color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

func (s Severity) String() string {
	switch s {
	case SeverityDebug:
		return "DEBUG"
	case SeverityInfo:
		return "INFO"
	case SeverityWarn:
		return "WARN"
	case SeverityError:
		return "ERROR"
	case SeverityFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// LogEvent logs an event with the given severity and message, associating it with the current span
// This is a better approach than mixing log and span APIs directly
func LogEvent(severity Severity, message string) {
	// Choose color based on severity
	var color string
	switch severity {
	case SeverityDebug:
		color = colorCyan
	case SeverityInfo:
		color = colorBlue
	case SeverityWarn:
		color = colorYellow
	case SeverityError:
		color = colorRed
	case SeverityFatal:
		color = colorRed
	default:
		color = colorWhite
	}

	fmt.Printf("[%s%v%s] %s\n", color, severity.String(), colorReset, message)
}

func LogInfo(message string) {
	LogEvent(SeverityInfo, message)
}

func LogWarning(message string) {
	LogEvent(SeverityWarn, message)
}

func LogError(err error, message string) {
	LogEvent(SeverityError, fmt.Sprintf("%s: %v", message, err))
}

func PanicError(err error, message string) {
	LogError(err, message)
	panic(fmt.Sprintf("%s: %v", message, err))
}

// PrintTrace prints the error trace to the console.
func PrintTrace(err error) {
	if err == nil {
		return
	}

	// Print the error message
	fmt.Printf("Error: %v\n", err)

	// Print the stack trace
	printStackTrace(err)

	fmt.Println() // Add some spacing
}

// ErrorTrace prints the error trace and exits the program.
func ErrorTrace(err error) {
	if err == nil {
		return
	}

	PrintTrace(err)
	os.Exit(1)
}

// printStackTrace attempts to print stack trace information.
func printStackTrace(err error) {
	fmt.Println("Stack trace:")

	// Get up to 10 stack frames
	pc := make([]uintptr, 10)
	n := runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc[:n])

	for {
		frame, more := frames.Next()
		// Skip runtime and standard library frames
		if !strings.Contains(frame.File, "runtime/") {
			file := removeParentFolder(frame.File)
			fmt.Printf("\t%s\n\t\t%s:%d\n", frame.Function, file, frame.Line)
		}

		if !more {
			break
		}
	}

	// If the error was wrapped, unwrap and show the underlying errors
	fmt.Println("Error chain:")
	var currentErr error = err
	for currentErr != nil {
		fmt.Printf("\t%v\n", currentErr)
		currentErr = errors.Unwrap(currentErr)
	}
}
