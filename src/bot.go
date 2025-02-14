package main

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"
	"wisbot/src/sqlgo"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/rotisserie/eris"
)

func StartBot() {
	discordSess, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		err = eris.Wrap(err, "Error while creating Discord session")
		ErrorTrace(err)
	}
	defer discordSess.Close()

	discordSess.AddHandler(messageCreate)

	discordSess.Identify.Intents = discordgo.IntentsAll

	// Open a websocket connection to Discord and begin listening.
	err2 := discordSess.Open()
	if err2 != nil {
		err2 = eris.Wrap(err2, "Error while opening Discord session")
		ErrorTrace(err2)
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

	var err error

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
			err := llmCommand(discordSess, messageProp, message)
			if err != nil {
				err = eris.Wrap(err, "Error while executing llm command")
				PrintTrace(err)
			}

		case "upload":
			err = uploadCommand(discordSess, messageProp, message)
			if err != nil {
				err = eris.Wrap(err, "Error while executing upload command")
				PrintTrace(err)
			}

		case "stats":
			err = statsCommand(discordSess, messageProp, message)
			if err != nil {
				err = eris.Wrap(err, "Error while executing stats command")
				PrintTrace(err)
			}

		case "help":
			helpCommand(discordSess, messageProp, message)
		}
	}
}

func helpCommand(discordSess *discordgo.Session, messageProperties *MessageProperties, message *discordgo.MessageCreate) {
	mess := `Commands:
		/wis llm <text> - Large Language Model
		/wis upload - Upload a file
		/wis help - Show this message
		/wis stats - Show some stats about the server
	`
	discordSess.ChannelMessageSend(messageProperties.ReplyChannelID, mess)
}

func llmCommand(discordSess *discordgo.Session, messageProperties *MessageProperties, message *discordgo.MessageCreate) error {

	chatMessages, _ := discordSess.ChannelMessages(message.ChannelID, 100, "", "", "")
	slices.Reverse(chatMessages)

	UserMessages := []UserMessage{}
	for _, msg := range chatMessages {
		UserMessages = append(UserMessages, UserMessage{UserName: msg.Author.Username, Content: msg.Content})
	}

	InputChatChannel <- UserMessages
	mess := <-OutputChatChannel

	log.Println("output mess:", mess)

	chunks, err := chunkDiscordMessage(mess, 1995)
	if err != nil {
		return eris.Wrap(err, "Error while chunking Discord message")
	}

	for _, message := range chunks {
		discordSess.ChannelMessageSend(messageProperties.ReplyChannelID, message)
		time.Sleep(200 * time.Millisecond)
	}

	return nil
}

func uploadCommand(discordSess *discordgo.Session, messageProperties *MessageProperties, message *discordgo.MessageCreate) error {
	uuid := uuid.New()

	// Count the number of files the user has uploaded.
	count, err := wisQueries.CountFilesFromUser(context.Background(), message.Author.Username)
	if err != nil {
		return eris.Wrap(err, "Error while executing CountFilesFromUser")
	}

	// Remove the oldest files if the user has uploaded too many.
	if count >= maxFilesPerUser {
		err1 := wisQueries.DeleteFileWhereUsersCountIsProvided(context.Background(),
			sqlgo.DeleteFileWhereUsersCountIsProvidedParams{
				DiscordUsername: message.Author.Username,
				Limit:           int32(count - maxFilesPerUser + 1),
			})
		if err1 != nil {
			return eris.Wrap(err1, "Error while executing DeleteFileWhereUsersCountIsProvided")
		}
	}

	// Insert the new file.
	err2 := wisQueries.InsertFile(context.Background(), sqlgo.InsertFileParams{
		ID:              uuid.String(),
		Uploaded:        false,
		DiscordUsername: message.Author.Username,
		Name:            "empty file",
	})
	if err2 != nil {
		return eris.Wrap(err2, "Error while executing InsertFile")
	}

	mess := fmt.Sprintf("Here is the link: https://%s/id/%s", serverIp, uuid.String())
	discordSess.ChannelMessageSend(message.ChannelID, mess)

	return nil
}

func statsCommand(discordSess *discordgo.Session, messageProperties *MessageProperties, message *discordgo.MessageCreate) error {
	guild, err := discordSess.Guild(message.GuildID)
	if err != nil {
		return eris.Wrap(err, "Error while executing Guild query")
	}

	memberCount := len(guild.Members) - 1        // Get the number of users in the discord server.
	serverCount := len(discordSess.State.Guilds) // Get the number of servers the bot is in.
	nsfwLevel := guild.NSFWLevel
	channelCount := len(guild.Channels) // Get the number of channels in the discord server.

	mess := fmt.Sprintf(
		"Stats:\n- Users: %d\n- Servers: %d\n- Channels: %d\n- NSFW Level: %d",
		memberCount, serverCount, channelCount, nsfwLevel,
	)

	discordSess.ChannelMessageSend(messageProperties.ReplyChannelID, mess)

	return nil
}
