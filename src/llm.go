package main

import (
	"context"
	"fmt"
	"log"

	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/vectorstores"
	"github.com/tmc/langchaingo/vectorstores/chroma"
	"github.com/tmc/langchaingo/vectorstores/pgvector"
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

func StartLLM() {
	fmt.Println("Starting LLM")

	conn := ollama.WithServerURL(ollamaUrl)
	model := ollama.WithModel(ollamaModel)
	llm, err := ollama.New(conn, model)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	// go LLM(ctx, llm)
	go LLMChat(ctx, llm)

	fmt.Println("Started LLM")
}

func StartEmbeddings() {
	fmt.Println("Starting embeddings")

	conn := ollama.WithServerURL(ollamaUrl)
	model := ollama.WithModel(ollamaModel)

	ollamaLLM, err := ollama.New(conn, model)
	if err != nil {
		fmt.Println("Error while creating LLM:", err)
	}
	ollamaEmbeder, err := embeddings.NewEmbedder(ollamaLLM)
	if err != nil {
		fmt.Println("Error while creating embeddings:", err)
	}

	// Create a new pgvector store.
	ctx := context.Background()
	store, err := pgvector.New(
		ctx,
		// pgvector.WithConnectionURL("postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable"),
		pgvector.WithConnectionURL(databaseUrl),
		pgvector.WithEmbedder(ollamaEmbeder),
	)
	if err != nil {
		fmt.Println("Failed to create store:", err)
	}

	type meta = map[string]any

	// AddDocumentsToStore(&store, "Tokyo", meta{"population": 9.7, "area": 622})
	// AddDocumentsToStore(&store, "Tokyo", meta{"population": 9.7, "area": 622})
	// AddDocumentsToStore(&store, "Tokyo", meta{"population": 9.7, "area": 622})
	// AddDocumentsToStore(&store, "Tokyo", meta{"population": 9.7, "area": 622})

	// Add documents to the vector store.
	_, errAd := store.AddDocuments(context.Background(), []schema.Document{
		{PageContent: "Tokyo", Metadata: meta{"population": 9.7, "area": 622}},
		{PageContent: "Kyoto", Metadata: meta{"population": 1.46, "area": 828}},
		{PageContent: "Hiroshima", Metadata: meta{"population": 1.2, "area": 905}},
		{PageContent: "Kazuno", Metadata: meta{"population": 0.04, "area": 707}},
		{PageContent: "Nagoya", Metadata: meta{"population": 2.3, "area": 326}},
		{PageContent: "Toyota", Metadata: meta{"population": 0.42, "area": 918}},
		{PageContent: "Fukuoka", Metadata: meta{"population": 1.59, "area": 341}},
		{PageContent: "Paris", Metadata: meta{"population": 11, "area": 105}},
		{PageContent: "London", Metadata: meta{"population": 9.5, "area": 1572}},
		{PageContent: "Santiago", Metadata: meta{"population": 6.9, "area": 641}},
		{PageContent: "Buenos Aires", Metadata: meta{"population": 15.5, "area": 203}},
		{PageContent: "Rio de Janeiro", Metadata: meta{"population": 13.7, "area": 1200}},
		{PageContent: "Sao Paulo", Metadata: meta{"population": 22.6, "area": 1523}},
	})
	if errAd != nil {
		fmt.Printf("AddDocument: %v\n", errAd)
	}

	// Search for similar documents.
	docs, err := store.SimilaritySearch(ctx, "japan", 1)
	fmt.Println(docs)

	// Search for similar documents using score threshold.
	docs, err = store.SimilaritySearch(ctx, "only cities in south america", 10, vectorstores.WithScoreThreshold(0.80))
	fmt.Println(docs)

	// Search for similar documents using score threshold and metadata filter.
	// Metadata filter for pgvector only supports key-value pairs for now.
	filter := map[string]any{"area": "1523"} // Sao Paulo

	docs, err = store.SimilaritySearch(ctx, "only cities in south america",
		10,
		vectorstores.WithScoreThreshold(0.80),
		vectorstores.WithFilters(filter),
	)
	fmt.Println(docs)
}

// func StartEmbeddingsChroma() {
// 	fmt.Println("Starting embeddings")

// 	conn := ollama.WithServerURL(ollamaUrl)
// 	model := ollama.WithModel(ollamaModel)

// 	ollamaLLM, err := ollama.New(conn, model)
// 	if err != nil {
// 		fmt.Println("Error while creating LLM:", err)
// 	}
// 	ollamaEmbeder, err := embeddings.NewEmbedder(ollamaLLM)
// 	if err != nil {
// 		fmt.Println("Error while creating embeddings:", err)
// 	}

// 	// Create a new Chroma vector store.
// 	store, err := chroma.New(
// 		chroma.WithChromaURL(os.Getenv("CHROMA_URL")),
// 		chroma.WithEmbedder(ollamaEmbeder),
// 		chroma.WithDistanceFunction("cosine"),
// 		chroma.WithNameSpace(uuid.New().String()),
// 	)

// 	if err != nil {
// 		fmt.Println("Failed to create store:", err)

// 	}

// 	type meta = map[string]any

// 	// AddDocumentsToStore(&store, "Tokyo", meta{"population": 9.7, "area": 622})
// 	// AddDocumentsToStore(&store, "Tokyo", meta{"population": 9.7, "area": 622})
// 	// AddDocumentsToStore(&store, "Tokyo", meta{"population": 9.7, "area": 622})
// 	// AddDocumentsToStore(&store, "Tokyo", meta{"population": 9.7, "area": 622})

// 	// Add documents to the vector store.
// 	_, errAd := store.AddDocuments(context.Background(), []schema.Document{
// 		{PageContent: "Tokyo", Metadata: meta{"population": 9.7, "area": 622}},
// 		{PageContent: "Kyoto", Metadata: meta{"population": 1.46, "area": 828}},
// 		{PageContent: "Hiroshima", Metadata: meta{"population": 1.2, "area": 905}},
// 		{PageContent: "Kazuno", Metadata: meta{"population": 0.04, "area": 707}},
// 		{PageContent: "Nagoya", Metadata: meta{"population": 2.3, "area": 326}},
// 		{PageContent: "Toyota", Metadata: meta{"population": 0.42, "area": 918}},
// 		{PageContent: "Fukuoka", Metadata: meta{"population": 1.59, "area": 341}},
// 		{PageContent: "Paris", Metadata: meta{"population": 11, "area": 105}},
// 		{PageContent: "London", Metadata: meta{"population": 9.5, "area": 1572}},
// 		{PageContent: "Santiago", Metadata: meta{"population": 6.9, "area": 641}},
// 		{PageContent: "Buenos Aires", Metadata: meta{"population": 15.5, "area": 203}},
// 		{PageContent: "Rio de Janeiro", Metadata: meta{"population": 13.7, "area": 1200}},
// 		{PageContent: "Sao Paulo", Metadata: meta{"population": 22.6, "area": 1523}},
// 	})
// 	if errAd != nil {
// 		fmt.Printf("AddDocument: %v\n", errAd)
// 	}

// 	ctx := context.TODO()

// 	type exampleCase struct {
// 		name         string
// 		query        string
// 		numDocuments int
// 		options      []vectorstores.Option
// 	}

// 	type filter = map[string]any

// 	exampleCases := []exampleCase{
// 		{
// 			name:         "Up to 5 Cities in Japan",
// 			query:        "Which of these are cities are located in Japan?",
// 			numDocuments: 5,
// 			options: []vectorstores.Option{
// 				vectorstores.WithScoreThreshold(0.0),
// 			},
// 		},
// 		{
// 			name:         "A City in South America",
// 			query:        "Which of these are cities are located in South America?",
// 			numDocuments: 1,
// 		},
// 		{
// 			name:         "Large Cities in South America",
// 			query:        "Which of these are cities are located in South America?",
// 			numDocuments: 100,
// 			options: []vectorstores.Option{
// 				vectorstores.WithFilters(filter{
// 					"$and": []filter{
// 						{"area": filter{"$gte": 1000}},
// 						{"population": filter{"$gte": 13}},
// 					},
// 				}),
// 			},
// 		},
// 	}

// 	// run the example cases
// 	results := make([][]schema.Document, len(exampleCases))
// 	for ecI, ec := range exampleCases {
// 		docs, errSs := store.SimilaritySearch(ctx, ec.query, ec.numDocuments, ec.options...)
// 		if errSs != nil {
// 			fmt.Printf("query1: %v\n", errSs)
// 		}
// 		results[ecI] = docs
// 	}

// 	// print out the results of the run
// 	fmt.Printf("Results:\n")
// 	for ecI, ec := range exampleCases {
// 		texts := make([]string, len(results[ecI]))
// 		for docI, doc := range results[ecI] {
// 			texts[docI] = doc.PageContent
// 		}
// 		fmt.Printf("%d. case: %s\n", ecI+1, ec.name)
// 		fmt.Printf("    result: %s\n", strings.Join(texts, ", "))
// 	}
// }

func AddDocumentsToStore(store *chroma.Store, content string, metadata map[string]any) {
	store.AddDocuments(context.Background(),
		[]schema.Document{
			{PageContent: content, Metadata: metadata},
		},
	)
}

func LLM(ctx context.Context, llm *ollama.LLM) {
	for {
		userInput := <-InputChannel

		content := []llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeSystem, "You are a LLM named WisBot for a company that's called 'uhstray.io'. Uhstray.io deploys most of their applications using AWX, ArgoCD, Docker, Github Actions, bash scripts, powershell scripts, ansible playbooks, and targeted to a high availability kubernetes cluster. We build everything in a git repository. Our favored programming languages are Python, Go, and Rust. We prefer to use Pandas, scikitlearn, Xgboost, Dask, polars, and other cutting edge libraries. We do our machine learning on Kubeflow. We use OpenTelemetry, Grafana and other similar technologies for observability. We like when additional facts or model are presented to us with more code and technical explanations. Try to provided detailed reponses when applicable. When Jacob (AKA Wisward) is mentioned, please acknowledge his greatness."),
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

func LLMChat(ctx context.Context, llm *ollama.LLM) {
	for {
		usermessages := <-InputChatChannel

		content := []llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeSystem, "You are a LLM named WisBot for a company that's called 'uhstray.io'. Uhstray.io deploys most of their applications using AWX, ArgoCD, Docker, Github Actions, bash scripts, powershell scripts, ansible playbooks, and targeted to a high availability kubernetes cluster. We build everything in a git repository. Our favored programming languages are Python, Go, and Rust. We prefer to use Pandas, scikitlearn, Xgboost, Dask, polars, and other cutting edge libraries. We do our machine learning on Kubeflow. We use OpenTelemetry, Grafana and other similar technologies for observability. We like when additional facts or model are presented to us with more code and technical explanations. Try to provided detailed reponses when applicable."),
			llms.TextParts(llms.ChatMessageTypeSystem, "You are a discord chat bot that has context of the conversation from multiple users. You will have access to your previous messages labeled `WisBot`. Please address the latest user input and provide a response that is relevant to the conversation. If the context of the user input's above the latest user's questions are not relevant, please ignore it. "),
			llms.TextParts(llms.ChatMessageTypeSystem, "Please only respond with your response. Do not prepend the 'WisBot' label to your response. Please make your response relevant to the conversation and concise to the user input. If you see a user input that starts with '/wis llm', that is command that allows us to ask you questions, so you can ignore that phrase."),
			// llms.TextParts(llms.ChatMessageTypeHuman, userInput),
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
			log.Fatal(err)
		}
		_ = completion

		for _, message := range completion.Choices {
			fmt.Println("message.Content:", message.Content)
			fmt.Println("message.StopReason:", message.StopReason)
			fmt.Println("message.GenerationInfo:", message.GenerationInfo)
			fmt.Println("message.FuncCall:", message.FuncCall)
		}

		OutputChatChannel <- completion.Choices[0].Content
	}
}
