package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"time"
	"wisbot/src/sqlgo"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

var (
	token   = os.Getenv("DISCORD_TOKEN_WISBOT")
	appID   = os.Getenv("DISCORD_APPLICATION_ID_WISBOT")
	guildID = "" // "889910011113906186"
)

func removeAllCommands(s *discordgo.Session) {
	registeredCommands, err := s.ApplicationCommands(appID, guildID)
	if err != nil {
		log.Fatalf("Could not fetch registered commands: %v", err)
	}

	for _, v := range registeredCommands {
		fmt.Println("Removing command: ", v.Name, v.Description)

		err2 := s.ApplicationCommandDelete(appID, guildID, v.ID)
		if err2 != nil {
			fmt.Printf("Cannot delete '%v' command: %v\n", v.Name, err2)
		}
	}
}

func createCommands(discordSess *discordgo.Session) {
	_, err := discordSess.ApplicationCommandBulkOverwrite(appID, "",
		[]*discordgo.ApplicationCommand{
			{
				Name:        "hello-world",
				Description: "Showcase of a basic slash command!",
			},
			{
				Name:        "llm",
				Description: "Large Language Model",
			},
		})
	if err != nil {
		log.Fatalf("Could not register commands: %v", err)
	}
}

func createCommandsHandler(discordSess *discordgo.Session) {
	discordSess.AddHandler(func(
		s *discordgo.Session,
		i *discordgo.InteractionCreate,
	) {
		data := i.ApplicationCommandData()

		fmt.Println("data.Name", data.Name)
		for i, v := range data.Options {
			fmt.Println("data.Options", i, v)
		}

		switch data.Name {

		case "hello-world":
			err := s.InteractionRespond(
				i.Interaction,
				&discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Hello world!",
					},
				},
			)
			if err != nil {
				fmt.Println("Error responding to hello-world command: ", err)
			}

		case "llm":
			err := s.InteractionRespond(
				i.Interaction,
				&discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Hello world!",
					},
				},
			)
			if err != nil {
				fmt.Println("Error responding to llm command: ", err)
			}

		}
	})
}

func CommandsStartBot() {
	discordSess, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalf("Could not create Discord session: %v", err)
	}

	removeAllCommands(discordSess)
	// createCommands(discordSess)
	// createCommandsHandler(discordSess)

	discordSess.Identify.Intents = discordgo.IntentsAll

	err2 := discordSess.Open()
	if err2 != nil {
		log.Fatalf("Error encountered while Opening Discord session. %v", err)
	}
	defer discordSess.Close()
}

//
//
//
//

func StartBot() {
	discordSess, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		log.Fatalf("Error while creating Discord session: %v", err)
	}
	defer discordSess.Close()

	discordSess.AddHandler(messageCreate)

	discordSess.Identify.Intents = discordgo.IntentsAll

	// Open a websocket connection to Discord and begin listening.
	err = discordSess.Open()
	if err != nil {
		log.Fatalf("Error encountered while opening Discord session. %v", err)
		return
	}
}

func onReady(discordSess *discordgo.Session, event *discordgo.Ready) {
	fmt.Printf("Logged in as: %v#%v \n", discordSess.State.User.Username, discordSess.State.User.Discriminator)

	// ServerID := "889910011113906186"
	ChannelID := "998632857306136617"

	messages, _ := discordSess.ChannelMessages(ChannelID, 100, "", "", "")

	slices.Reverse(messages)
	for _, msg := range messages {
		fmt.Printf("%s : %s\n", msg.Author.Username, msg.Content)
	}
}

func printMessage(discordSess *discordgo.Session, message *discordgo.MessageCreate) {
	// timestamp := message.Timestamp.Local().Format("2006-09-25 03:04:05 PM")

	channel, err := discordSess.Channel(message.ChannelID)
	if err != nil {
		fmt.Printf("Error: Could not retreive channel. %v \n", err)
	}

	guild, err2 := discordSess.Guild(message.GuildID)
	if err2 != nil {
		// fmt.Printf("Error: Could not retreive guild. %v \n", err2)
		guild = &discordgo.Guild{Name: "Private Message"}
	}

	fmt.Printf("Server: %s (Channel: %s)\n", guild.Name, channel.Name)
	fmt.Printf("Username: %s - %s\n", message.Author.GlobalName, message.Content)
}

type MessageProperties struct {
	IsPrivate bool
	IsCommand bool

	ReplyChannelID string
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func messageCreate(discordSess *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Author.ID == discordSess.State.User.ID {
		return
	}

	printMessage(discordSess, message)

	messageProp := &MessageProperties{
		IsPrivate:      false,
		IsCommand:      false,
		ReplyChannelID: message.ChannelID,
	}

	// Check if the message is a command
	head, tail := nextToken(message.Content)
	message.Content = tail
	switch head {
	case "/wis", "/wis?", "/wisbot", "/wisbot?":
		if strings.HasSuffix(head, "?") {
			messageProp.IsPrivate = true
			userChannel, _ := discordSess.UserChannelCreate(message.Author.ID)
			messageProp.ReplyChannelID = userChannel.ID
		}
		messageProp.IsCommand = true

		// Handle the command
		head, tail = nextToken(tail)
		message.Content = tail

		switch head {
		case "llm":
			llmCommand(discordSess, messageProp, message)
			return

		case "upload":
			uploadCommand(discordSess, messageProp, message)
			return

		case "help":
			helpCommand(discordSess, messageProp, message)
			return
		}
	}
}

func helpCommand(discordSess *discordgo.Session, messageProperties *MessageProperties, message *discordgo.MessageCreate) {
	mess := `Commands:
		/wis llm <text> - Large Language Model
		/wis upload - Upload a file
		/wis help - Show this message
	`
	discordSess.ChannelMessageSend(messageProperties.ReplyChannelID, mess)
}

func llmCommand(discordSess *discordgo.Session, messageProperties *MessageProperties, message *discordgo.MessageCreate) {

	chatMessages, _ := discordSess.ChannelMessages(message.ChannelID, 100, "", "", "")
	slices.Reverse(chatMessages)

	UserMessages := []UserMessage{}
	for _, msg := range chatMessages {
		UserMessages = append(UserMessages, UserMessage{UserName: msg.Author.Username, Content: msg.Content})
	}

	InputChatChannel <- UserMessages
	output := <-OutputChatChannel

	mess := output //fmt.Sprintf("LLM: %s", output)

	chunks := chunkDiscordMessage(mess, 1995)
	for _, message := range chunks {
		discordSess.ChannelMessageSend(messageProperties.ReplyChannelID, message)
		time.Sleep(200 * time.Millisecond)
	}
}

func uploadCommand(discordSess *discordgo.Session, messageProperties *MessageProperties, message *discordgo.MessageCreate) {
	uuid := uuid.New()

	// Count the number of files the user has uploaded.
	count, err := wisQueries.CountFilesFromUser(context.Background(), message.Author.Username)
	if err != nil {
		fmt.Println("Error while executing CountFilesFromUser", err.Error())
		return
	}

	// Remove the oldest files if the user has uploaded too many.
	if count >= maxFilesPerUser {
		err1 := wisQueries.DeleteFileWhereUsersCountIsProvided(context.Background(),
			sqlgo.DeleteFileWhereUsersCountIsProvidedParams{
				DiscordUsername: message.Author.Username,
				Limit:           int32(count - maxFilesPerUser + 1),
			})
		if err1 != nil {
			fmt.Println("Error while executing DeleteFileWhereUsersCountIsProvided query", err1.Error())
		}
	}

	// Insert the new file.
	err2 := wisQueries.InsertFile(context.Background(),
		sqlgo.InsertFileParams{
			ID:              uuid.String(),
			Uploaded:        false,
			DiscordUsername: message.Author.Username,
			Name:            "empty file",
		})
	if err2 != nil {
		fmt.Println("Error while executing InsertFile query", err2.Error())
	}

	mess := fmt.Sprintf("Here is the link: http://%s:%s/id/%s", serverIp, serverPort, uuid.String())
	discordSess.ChannelMessageSend(message.ChannelID, mess)
}
