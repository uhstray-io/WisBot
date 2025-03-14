package main

import (
	"fmt"
	"strings"
)

// chunkDiscordMessage splits a message into chunks that respect Discord's message length limits,
// while preserving code blocks. Code blocks are kept intact in their own chunks.
// Parameters:
//   - input: The string to be chunked
//   - maxLength: The maximum length of each chunk
//
// Returns:
//   - []string: Array of chunks
//   - error: Error if any chunk exceeds the maximum length
func chunkDiscordMessage(input string, maxLength int) ([]string, error) {
	if input == "" {
		return []string{}, nil
	}

	// First separate code blocks from regular text
	chunks, isCodeBlock := splitCodeBlocks(input)
	var result []string

	for i, chunk := range chunks {
		if isCodeBlock[i] {
			// Check if the code block is already too long
			if len(chunk) > maxLength {
				return nil, fmt.Errorf("code block too large: %d > %d", len(chunk), maxLength)
			}
			// Keep code blocks intact
			result = append(result, chunk)
		} else {
			// Split text chunks by newlines and then by length
			textChunks := splitStringByLength(chunk, maxLength)
			result = append(result, textChunks...)
		}
	}

	// Final validation to ensure no chunk exceeds Discord's limit
	for _, chunk := range result {
		if len(chunk) > maxLength {
			return nil, fmt.Errorf("chunk exceeds maximum length. %v > %v", len(chunk), maxLength)
		}
	}

	return result, nil
}

// splitStringByLength splits a string into a list of smaller strings of a given length.
// We prefer to prefer to split the string at a Code Blocks, New Lines, and Spaces, in that order.
// Parameters:
//   - input: The string to be split
//   - maxLength: The maximum length of each resulting chunk
//
// Returns:
//   - []string: Array of chunks, each respecting the maxLength constraint
func splitStringByLength(input string, maxLength int) []string {
	var chunks []string
	var currentChunk string
	// Split input by newlines to process line by line
	lines := strings.Split(input, "\n")

	for _, line := range lines {
		// If adding this line would exceed maxLength, start a new chunk
		if len(currentChunk)+len(line)+1 > maxLength {
			chunks = append(chunks, currentChunk)
			currentChunk = line
		} else {
			// Add a newline before appending the next line (except for the first line)
			if currentChunk != "" {
				currentChunk += "\n"
			}
			currentChunk += line
		}
	}

	// Add the last chunk if it's not empty
	if currentChunk != "" {
		chunks = append(chunks, currentChunk)
	}

	return chunks
}

// splitCodeBlocks splits a string into a list of smaller strings.
// Each string, if it contains a code block should the normal string from the codeblock portion.
// Parameters:
//   - input: The string to be split
//
// Returns:
//   - []string: Array of chunks, alternating between normal text and code blocks
//   - []bool: Parallel array indicating whether each chunk is a code block (true) or regular text (false)
func splitCodeBlocks(input string) ([]string, []bool) {
	var chunks []string
	var codeChunks []bool

	for {
		// Find the start of a code block marked by ```
		start := strings.Index(input, "```")
		if start == -1 {
			break
		}

		// Find the end of the code block (skip the first 3 chars which are ```)
		end := strings.Index(input[start+3:], "```")
		if end == -1 {
			break
		}

		// Add the text before the code block if it exists
		if start > 0 {
			chunks = append(chunks, input[:start])
			codeChunks = append(codeChunks, false)
		}

		// Add the code block including the ``` markers
		chunks = append(chunks, input[start:end+3])
		codeChunks = append(codeChunks, true)

		// Continue with the rest of the string after the code block
		if end+3 < len(input) {
			input = input[end+3:]
		}
	}

	// Add any remaining text after the last code block
	if input != "" {
		chunks = append(chunks, input)
		codeChunks = append(codeChunks, false)
	}

	// Removed extra new lines from each chunk
	for i, chunk := range chunks {
		chunks[i] = strings.Trim(chunk, "\n")
	}

	return chunks, codeChunks
}

// nextToken extracts the first word from a string and returns both the word and the remainder of the string.
// Parameters:
//   - s: The input string
//
// Returns:
//   - string: The first word (token)
//   - string: The remainder of the string after removing the first word
// func nextToken(s string) (string, string) {
// 	if len(s) == 0 {
// 		return "", "" // Handle empty string case
// 	}

// 	s = strings.TrimSpace(s)            // Remove leading/trailing whitespace
// 	tokens := strings.SplitN(s, " ", 2) // Split into at most 2 parts at the first space

// 	if len(tokens) == 1 {
// 		return tokens[0], "" // Only one token, no remainder
// 	}

// 	return tokens[0], tokens[1] // Return first token and the rest
// }

// peekNextToken returns the first word from a string without modifying the original string.
// Parameters:
//   - s: The input string
//
// Returns:
//   - string: The first word (token)
// func peekNextToken(s string) string {
// 	head, _ := nextToken(s)
// 	return head
// }
