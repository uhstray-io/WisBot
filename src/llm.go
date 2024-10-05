package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"

	"github.com/shirou/gopsutil/process"
)

var InputChannel = make(chan string)
var OutputChannel = make(chan string)

func StartLLM() {
	StartOllama()

	PullImage(config.LLM.Name)

	model := ollama.WithModel(config.LLM.Name)
	llm, err := ollama.New(model)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	go LLM(ctx, llm)
}

// StartOllama starts the ollama service if it's not already running
func StartOllama() {
	// Collect the currently running processes
	processes, err := process.Processes()
	if err != nil {
		log.Fatal(err)
	}

	// Check if Ollama is running
	IsOllamaRunning := false
	for _, process := range processes {
		name, err := process.Name()
		if err != nil {
			log.Fatal(err)
		}

		if strings.Contains(name, "ollama") {
			IsOllamaRunning = true
			break
		}
	}

	// Start Ollama if it's not running
	if !IsOllamaRunning {
		fmt.Println("Starting Ollama")

		err := exec.Command("ollama", "serve").Start()
		if err != nil {
			log.Fatal(err)
		}

		time.Sleep(5 * time.Second)
	}
}

// PullImage pulls the LLM model image from the Ollama repository
func PullImage(imageName string) {
	// Check if the image already exists
	out, err := exec.Command("ollama", "list").Output()
	if err != nil {
		log.Fatal(err)
	}

	// If the image already exists, skip pulling
	if strings.Contains(string(out), imageName) {
		fmt.Printf("Image `%s` already exists\n", imageName)
		return
	}

	// Example Command : ollama pull llama3.2
	fmt.Printf("Pulling image `%s`\n", imageName)
	cmd := exec.Command("ollama", "pull", imageName)

	// Set the output to be written to stdout
	var buf bytes.Buffer
	mw := io.MultiWriter(os.Stdout, &buf)
	cmd.Stdout = mw
	cmd.Stderr = mw

	// Print the output of the command
	go func() {
		for {
			if len(buf.Bytes()) > 0 {
				fmt.Printf("%s", buf.String())
				buf.Reset()
			}
			if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
				break
			}
		}
	}()

	// Execute the command
	err2 := cmd.Start()
	if err2 != nil {
		log.Fatal(err2)
	}

	cmd.Wait()
}

func LLM(ctx context.Context, llm *ollama.LLM) {
	for {
		userInput := <-InputChannel

		content := []llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeSystem, "You are a LLM for a company that's called 'uhstray.io'. Uhstray.io deploys most of their applications using AWX, ArgoCD, Docker, Github Actions, bash scripts, powershell scripts, ansible playbooks, and targeted to a high availability kubernetes cluster. We build everything in a git repository. Our favored programming languages are python, go, and rust. We prefer to use pandas, scikitlearn, rust, polars, and other cutting edge libraries. We do our machine learning on Kubeflow. We use OpenTelemetry, Grafana and other similar technologies for observability. We like when additional facts or options are presented to us with more code and technical explanations. Try to provided detailed reponses when applicable. When Jacob (AKA Wisward) is mentioned, please acknowledge his greatness."),
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
