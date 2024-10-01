package main

import (
	"strings"
)

func chunkDiscordMessage(input string, maxLength int) []string {
	var newChunks []string
	chunks, bools := chunkCodeBlock(input)

	for i, chunk := range chunks {
		if bools[i] {
			// Code Chunks should just be moved to the newChunks
			newChunks = append(newChunks, chunk)
		} else {
			// Other chunks should be split into smaller chunks
			newChunks = append(newChunks, chunkString(chunk, maxLength)...)
		}
	}

	return newChunks
}

// chunkString splits a string into a list of smaller strings of a given length.
// We prefer to prefer to split the string at a Code Blocks, New Lines, and Spaces, in that order.
func chunkString(input string, maxLength int) []string {
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

// chunkCodeBlock splits a string into a list of smaller strings.
// Each string, if it contains a code block should the normal string from the codeblock portion.
func chunkCodeBlock(input string) ([]string, []bool) {
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
		end += start + 3

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
