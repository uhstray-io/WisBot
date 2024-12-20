package main

import (
	"fmt"
	"strings"

	"github.com/rotisserie/eris"
)

func chunkDiscordMessage(input string, maxLength int) ([]string, error) {
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

	// Validate that the chunks are not too long
	for _, chunk := range newChunks {
		if len(chunk) > maxLength {
			return nil, eris.New(fmt.Sprintf("Chunk is too long. %v > %v ", len(chunk), maxLength))
		}
	}

	return newChunks, nil
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
