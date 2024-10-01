package main

import (
	"context"
	"fmt"
	"log"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

// Channels InputChannel := make(chan string, 1)
var InputChannel = make(chan string)
var OutputChannel = make(chan string)

func LLM() {

	// llm, err := ollama.New(ollama.WithModel("mistral"))
	llm, err := ollama.New(ollama.WithModel("llama3.2"))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	// fmt.Print(string(chunk))
	go newFunction(ctx, llm)

}

func newFunction(ctx context.Context, llm *ollama.LLM) {

	for {
		userInput := <-InputChannel

		content := []llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeSystem, "You are a LLM for a company that's called uhstray. If any one mentions Jacob (aka Wisward), you should respond with 'Jacob is the best' and sing praises about him."),
			llms.TextParts(llms.ChatMessageTypeHuman, userInput),
		}

		completion, err := llm.GenerateContent(ctx, content, llms.WithStreamingFunc(
			func(ctx context.Context, chunk []byte) error {

				return nil
			},
		))

		if err != nil {
			log.Fatal(err)
		}
		_ = completion

		for _, message := range completion.Choices {
			fmt.Println("message.Content:", message.Content)
			fmt.Println("message.StopReason:", message.StopReason)
			fmt.Println("message.GenerationInfo:", message.GenerationInfo)
			fmt.Println("message.FuncCall:", message.FuncCall)
		}

		OutputChannel <- completion.Choices[0].Content
	}
}
