package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

var InputChannel = make(chan string)
var OutputChannel = make(chan string)

func StartLLM() {
	fmt.Println("Starting LLM")

	conn := ollama.WithServerURL(os.Getenv("OLLAMA_URL"))
	model := ollama.WithModel(os.Getenv("OLLAMA_MODEL"))
	llm, err := ollama.New(conn, model)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	go LLM(ctx, llm)

	fmt.Println("Started LLM")
}

func LLM(ctx context.Context, llm *ollama.LLM) {
	for {
		userInput := <-InputChannel

		content := []llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeSystem, "You are a LLM for a company that's called 'uhstray.io'. Uhstray.io deploys most of their applications using AWX, ArgoCD, Docker, Github Actions, bash scripts, powershell scripts, ansible playbooks, and targeted to a high availability kubernetes cluster. We build everything in a git repository. Our favored programming languages are python, go, and rust. We prefer to use pandas, scikitlearn, rust, polars, and other cutting edge libraries. We do our machine learning on Kubeflow. We use OpenTelemetry, Grafana and other similar technologies for observability. We like when additional facts or model are presented to us with more code and technical explanations. Try to provided detailed reponses when applicable. When Jacob (AKA Wisward) is mentioned, please acknowledge his greatness."),
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
