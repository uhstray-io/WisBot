package main

import (
	"context"
	"fmt"
	"log"
	"slices"
	"time"
	"wisbot/src/sqlgo"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "wis",
		Description: "Main WisBot command",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "llm",
				Description: "Interact with the Large Language Model",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "text",
						Description: "Text to send to the LLM",
						Required:    true,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "upload",
				Description: "Upload a file",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "help",
				Description: "Show help message",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "stats",
				Description: "Show server statistics",
			},
		},
	},
}

func StartBot() {
	discordSess, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		err = fmt.Errorf("error while creating Discord session: %w", err)
		ErrorTrace(err)
	}
	defer discordSess.Close()

	discordSess.AddHandler(messageCreate)
	discordSess.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Println("Bot is ready! Logged in as:", s.State.User.Username)
		registerCommands(s)
	})
	discordSess.AddHandler(interactionCreate)

	discordSess.Identify.Intents = discordgo.IntentsAll

	err2 := discordSess.Open()
	if err2 != nil {
		err2 = fmt.Errorf("error while opening Discord session: %w", err2)
		ErrorTrace(err2)
	}
}

func registerCommands(s *discordgo.Session) {
	for _, command := range commands {
		_, err := s.ApplicationCommandCreate(s.State.User.ID, "", command)
		if err != nil {
			log.Printf("Error creating command %v: %v", command.Name, err)
		}
	}
	log.Println("Slash commands registered successfully")
}

func onReady(discordSess *discordgo.Session, event *discordgo.Ready) {
	fmt.Printf("Logged in as: %v#%v \n", discordSess.State.User.Username, discordSess.State.User.Discriminator)

	ChannelID := "998632857306136617"

	messages, _ := discordSess.ChannelMessages(ChannelID, 100, "", "", "")

	slices.Reverse(messages)
	for _, msg := range messages {
		fmt.Printf("%s : %s\n", msg.Author.Username, msg.Content)
	}
}

func printMessage(discordSess *discordgo.Session, message *discordgo.MessageCreate) {
	channel, err := discordSess.Channel(message.ChannelID)
	if err != nil {
		fmt.Printf("Error: Could not retrieve channel. %v \n", err)
	}

	guild, err2 := discordSess.Guild(message.GuildID)
	if err2 != nil {
		guild = &discordgo.Guild{Name: "Private Message"}
	}

	fmt.Printf("Server: %s (Channel: %s)\n", guild.Name, channel.Name)
	fmt.Printf("Username: %s - %s\n", message.Author.GlobalName, message.Content)
}

type MessageProperties struct {
	IsPrivate      bool
	IsCommand      bool
	ReplyChannelID string
}

// Legacy message handler - keeping for backward compatibility
func messageCreate(discordSess *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Author.ID == discordSess.State.User.ID {
		return
	}

	printMessage(discordSess, message)
}

func interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := i.ApplicationCommandData()

	if data.Name != "wis" {
		return
	}

	// Get the subcommand
	subcommand := data.Options[0].Name
	options := data.Options[0].Options

	switch subcommand {
	case "llm":
		handleLLMCommand(s, i, options)
	case "upload":
		handleUploadCommand(s, i)
	case "stats":
		handleStatsCommand(s, i)
	case "help":
		handleHelpCommand(s, i)
	}
}

func handleHelpCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	helpText := `Commands:
- /wis llm [text] - Large Language Model
- /wis upload - Upload a file
- /wis help - Show this message
- /wis stats - Show some stats about the server`

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: helpText,
		},
	})
}

func handleLLMCommand(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	// Acknowledge the interaction to prevent timeout
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	text := options[0].StringValue()

	chatMessages, _ := s.ChannelMessages(i.ChannelID, 100, "", "", "")
	slices.Reverse(chatMessages)

	UserMessages := []UserMessage{}
	for _, msg := range chatMessages {
		UserMessages = append(UserMessages, UserMessage{UserName: msg.Author.Username, Content: msg.Content})
	}

	// Add the current command as a message
	UserMessages = append(UserMessages, UserMessage{
		UserName: getUserFromInteraction(i),
		Content:  "/wis llm " + text,
	})

	InputChatChannel <- UserMessages
	response := <-OutputChatChannel

	log.Println("LLM response:", response)

	chunks, err := chunkDiscordMessage(response, 1995)
	if err != nil {
		log.Printf("Error chunking message: %v", err)

		str := fmt.Sprintf("Error: %v", err)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &str,
		})
		return
	}

	// Send the first chunk as the interaction response
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &chunks[0],
	})

	// Send additional chunks as follow-up messages
	for _, chunk := range chunks[1:] {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: chunk,
		})
		time.Sleep(200 * time.Millisecond)
	}
}

// getUserFromInteraction extracts the username from the interaction
// The member who invoked this interaction. NOTE: the Member field is only filled when the slash command was invoked in a guild; if it was invoked in a DM, the `User` field will be filled instead. Make sure to check for `nil` before using this field.
func getUserFromInteraction(i *discordgo.InteractionCreate) string {
	if i.Member != nil {
		return i.Member.User.Username
	}
	return i.User.Username
}

func handleUploadCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	uuid := uuid.New()
	username := getUserFromInteraction(i)

	// Count the number of files the user has uploaded
	count, err := wisQueries.CountFilesFromUser(context.Background(), username)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Error: %v", err),
			},
		})
		return
	}

	// Remove the oldest files if the user has uploaded too many
	if count >= maxFilesPerUser {
		err := wisQueries.DeleteFileWhereUsersCountIsProvided(context.Background(),
			sqlgo.DeleteFileWhereUsersCountIsProvidedParams{
				DiscordUsername: username,
				Limit:           int32(count - maxFilesPerUser + 1),
			})
		if err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Error cleaning up old files: %v", err),
				},
			})
			return
		}
	}

	// Insert the new file
	err = wisQueries.InsertFile(context.Background(), sqlgo.InsertFileParams{
		ID:              uuid.String(),
		Uploaded:        false,
		DiscordUsername: username,
		Name:            "empty file",
	})
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Error creating file entry: %v", err),
			},
		})
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Here is the link: https://%s/id/%s", serverIp, uuid.String()),
		},
	})
}

func handleStatsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	guild, err := s.Guild(i.GuildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Error getting guild info: %v", err),
			},
		})
		return
	}

	memberCount := len(guild.Members) - 1 // Get the number of users in the discord server
	serverCount := len(s.State.Guilds)    // Get the number of servers the bot is in
	nsfwLevel := guild.NSFWLevel
	channelCount := len(guild.Channels) // Get the number of channels in the discord server

	stats := fmt.Sprintf(
		"Stats:\n- Users: %d\n- Servers: %d\n- Channels: %d\n- NSFW Level: %d",
		memberCount, serverCount, channelCount, nsfwLevel,
	)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: stats,
		},
	})
}
