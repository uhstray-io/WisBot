package main

import (
	"path/filepath"
	"strings"
)

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
