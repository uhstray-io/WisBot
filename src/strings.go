package main

import (
	"errors"
	"fmt"
	"strings"
)

// chunkDiscordMessage splits a message into chunks that respect Discord's message length limits,
// while preserving code blocks. Code blocks are kept intact in their own chunks.
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
				return nil, errors.New(fmt.Sprintf("Code block too large: %d > %d", len(chunk), maxLength))
			}
			// Keep code blocks intact
			result = append(result, chunk)
		} else {
			// Split text chunks by newlines and then by length
			textChunks := splitStringByLength(chunk, maxLength)
			result = append(result, textChunks...)
		}
	}

	// Final validation
	for _, chunk := range result {
		if len(chunk) > maxLength {
			return nil, errors.New(fmt.Sprintf("Chunk exceeds maximum length. %v > %v ", len(chunk), maxLength))
		}
	}

	return result, nil
}

// splitStringByLength splits a string into a list of smaller strings of a given length.
// We prefer to prefer to split the string at a Code Blocks, New Lines, and Spaces, in that order.
func splitStringByLength(input string, maxLength int) []string {
	var chunks []string
	var currentChunk string
	lines := strings.Split(input, "\n")

	for _, line := range lines {
		if len(currentChunk)+len(line)+1 > maxLength {
			chunks = append(chunks, currentChunk)
			currentChunk = line
		} else {
			if currentChunk != "" {
				currentChunk += "\n"
			}
			currentChunk += line
		}
	}

	if currentChunk != "" {
		chunks = append(chunks, currentChunk)
	}

	return chunks
}

// splitCodeBlocks splits a string into a list of smaller strings.
// Each string, if it contains a code block should the normal string from the codeblock portion.
func splitCodeBlocks(input string) ([]string, []bool) {
	var chunks []string
	var codeChunks []bool

	for {
		start := strings.Index(input, "```")
		if start == -1 {
			break
		}
		end := strings.Index(input[start+3:], "```")
		if end == -1 {
			break
		}

		if start > 0 {
			chunks = append(chunks, input[:start])
			codeChunks = append(codeChunks, false)
		}

		chunks = append(chunks, input[start:end+3])
		codeChunks = append(codeChunks, true)

		if end+3 < len(input) {
			input = input[end+3:]
		}
	}

	if input != "" {
		chunks = append(chunks, input)
		codeChunks = append(codeChunks, false)
	}

	// Removed extra new lines
	for i, chunk := range chunks {
		chunks[i] = strings.Trim(chunk, "\n")
	}

	return chunks, codeChunks
}

func nextToken(s string) (string, string) {
	if len(s) == 0 {
		return "", ""
	}

	s = strings.TrimSpace(s)
	tokens := strings.SplitN(s, " ", 2)

	if len(tokens) == 1 {
		return tokens[0], ""
	}

	return tokens[0], tokens[1]
}

func peekNextToken(s string) string {
	head, _ := nextToken(s)
	return head
}
