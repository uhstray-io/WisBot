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

var InputChatChannel = make(chan []UserMessage)
var OutputChatChannel = make(chan string)

type UserMessage struct {
	UserName string
	Content  string
}

func (um *UserMessage) Format() string {
	return fmt.Sprintf("%s: %s\n", um.UserName, um.Content)
}

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

	// Start LLMChat in a separate goroutine
	go func() {
		if err := LLMChat(ctx, llm); err != nil {
			ErrorTrace(fmt.Errorf("error while running LLMChat: %w", err))
		}
	}()
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

func LLMChat(ctx context.Context, llm *ollama.LLM) error {
	for {
		usermessages := <-InputChatChannel

		ctx, span := StartSpan(ctx, "LLMChat.GenerateContent")
		span.SetAttributes(
			attribute.Int("message.count", len(usermessages)),
			attribute.String("model", ollamaModel),
		)

		content := []llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeSystem, "You are a LLM named WisBot for a company that's called 'uhstray.io'. Uhstray.io deploys most of their applications using AWX, ArgoCD, Docker, Github Actions, bash scripts, powershell scripts, ansible playbooks, and targeted to a high availability kubernetes cluster. We build everything in a git repository. Our favored programming languages are Python, Go, and Rust. We prefer to use Pandas, scikitlearn, Xgboost, Dask, polars, and other cutting edge libraries. We do our machine learning on Kubeflow. We use OpenTelemetry, Grafana and other similar technologies for observability. We like when additional facts or model are presented to us with more code and technical explanations. Try to provided detailed reponses when applicable."),
			llms.TextParts(llms.ChatMessageTypeSystem, "You are a discord chat bot that has context of the conversation from multiple users. You will have access to your previous messages labeled `WisBot`. Please address the latest user input and provide a response that is relevant to the conversation. If the context of the user input's above the latest user's questions are not relevant, please ignore it. "),
			llms.TextParts(llms.ChatMessageTypeSystem, "Please only respond with your response. Do not prepend the 'WisBot' label to your response. Please make your response relevant to the conversation and concise to the user input. If you see a user input that starts with '/wis llm', that is command that allows us to ask you questions, so you can ignore that phrase."),
		}

		for _, usermessage := range usermessages {
			content = append(content, llms.TextParts(llms.ChatMessageTypeHuman, usermessage.Format()))
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

		OutputChatChannel <- completion.Choices[0].Content
	}
}
