package main

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
)

var InputChannel = make(chan string, 10)  // Buffered to prevent blocking
var OutputChannel = make(chan string, 10) // Buffered to prevent blocking

func StartLLMService(ctx context.Context) {
	ctx, span := StartSpan(ctx, "StartLLM")
	defer span.End()

	LogInfo(ctx, "Starting LLM")

	conn := ollama.WithServerURL(ollamaUrl)
	model := ollama.WithModel(ollamaModel)
	llm, err := ollama.New(conn, model)
	if err != nil {
		PanicError(ctx, err, "Error while creating LLM")
		return
	}

	// Start LLM in a separate goroutine
	go func() {
		if err := LLM(ctx, llm); err != nil {
			PanicError(ctx, err, "Error while running LLM")
		}
	}()
}

func LLM(ctx context.Context, llm *ollama.LLM) error {
	for {
		select {
		case <-ctx.Done():
			LogInfo(ctx, "LLM service shutting down due to context cancellation.")
			return ctx.Err()
		case userInput := <-InputChannel:
			// Create a new context for this specific request to avoid carrying cancellation from previous requests.
			reqCtx, span := StartSpan(context.Background(), "LLM.GenerateContent") // Use context.Background() or a fresh parent context
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
					// If streaming is desired in the future, send chunks to OutputChannel here.
					// For now, the entire response is sent after generation.
					return nil
				},
			))

			if err != nil {
				span.RecordError(err)
				LogError(reqCtx, err, "LLM content generation failed")
				OutputChannel <- "Sorry, I encountered an error while generating a response."
				span.End()
				continue // Continue to next input
			}

			if len(completion.Choices) == 0 {
				LogEvent(reqCtx, log.SeverityWarn, "LLM returned no choices")
				OutputChannel <- "Sorry, I could not generate a response."
				span.End()
				continue
			}

			span.SetAttributes(
				attribute.String("output.length", fmt.Sprintf("%d", len(completion.Choices[0].Content))),
				attribute.String("stop.reason", string(completion.Choices[0].StopReason)),
			)
			span.End()

			OutputChannel <- completion.Choices[0].Content
		}
	}
}
