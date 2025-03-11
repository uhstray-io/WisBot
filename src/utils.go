package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

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

// removeParentFolder removes the parent path from the file path
// This is the same as the original function
func removeParentFolder(parentfolder string) string {
	SplitLabel := "Wisbot" // Change this to the parent directory name
	SplitPath := strings.Split(parentfolder, SplitLabel)

	if len(SplitPath) == 1 {
		return filepath.Base(parentfolder)
	}

	return SplitPath[1]
}
