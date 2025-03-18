package main

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"go.opentelemetry.io/otel/attribute"
)

var InputChannel = make(chan string)
var OutputChannel = make(chan string)

func StartLLM(ctx context.Context) {
	ctx, span := StartSpan(ctx, "StartLLM")
	defer span.End()

	fmt.Println("Starting LLM")

	conn := ollama.WithServerURL(ollamaUrl)
	model := ollama.WithModel(ollamaModel)

	llm, err := ollama.New(conn, model)
	if err != nil {
		err = fmt.Errorf("error while creating LLM: %w", err)
		ErrorTrace(err)
		return
	}

	// Start LLM in a separate goroutine
	go func() {
		if err := LLM(ctx, llm); err != nil {
			ErrorTrace(fmt.Errorf("error while running LLM: %w", err))
		}
	}()

	// LLMChat goroutine removed
}

func LLM(ctx context.Context, llm *ollama.LLM) error {
	for {
		userInput := <-InputChannel

		ctx, span := StartSpan(ctx, "LLM.GenerateContent")
		span.SetAttributes(
			attribute.String("input.length", fmt.Sprintf("%d", len(userInput))),
			attribute.String("model", ollamaModel),
		)

		content := []llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeSystem, "You are a LLM named WisBot for a company that's called 'uhstray.io'. Uhstray.io deploys most of their applications using AWX, ArgoCD, Docker, Github Actions, bash scripts, powershell scripts, ansible playbooks, and targeted to a high availability kubernetes cluster. We build everything in a git repository. Our favored programming languages are Python, Go, and Rust. We prefer to use Pandas, scikitlearn, Xgboost, Dask, polars, and other cutting edge libraries. We do our machine learning on Kubeflow. We use OpenTelemetry, Grafana and other similar technologies for observability. We like when additional facts or model are presented to us with more code and technical explanations. Try to provided detailed reponses when applicable.."),
			llms.TextParts(llms.ChatMessageTypeHuman, userInput),
		}

		completion, err := llm.GenerateContent(ctx, content, llms.WithStreamingFunc(
			func(ctx context.Context, chunk []byte) error {
				return nil
			},
		))

		if err != nil {
			span.RecordError(err)
			span.End()
			return fmt.Errorf("error while generating content: %w", err)
		}

		span.SetAttributes(
			attribute.String("output.length", fmt.Sprintf("%d", len(completion.Choices[0].Content))),
			attribute.String("stop.reason", string(completion.Choices[0].StopReason)),
		)
		span.End()

		for _, message := range completion.Choices {
			fmt.Println("message.Content:", message.Content)
			fmt.Println("message.StopReason:", message.StopReason)
			fmt.Println("message.GenerationInfo:", message.GenerationInfo)
			fmt.Println("message.FuncCall:", message.FuncCall)
		}

		OutputChannel <- completion.Choices[0].Content
	}
}

// LLMChat function removed
