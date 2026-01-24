package main

import (
	"context"
	"fmt"
	"slices"
	"time"
	"wisbot/src/sqlc"

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

func StartDiscordService(ctx context.Context) {
	LogInfo("Starting Discord bot")

	discordSess, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		PanicError(err, "Error while creating Discord session")
	}
	defer discordSess.Close()

	discordSess.AddHandler(messageCreate)
	discordSess.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		LogInfo("Bot ready")
		registerCommands(s)
	})
	discordSess.AddHandler(interactionCreate)

	discordSess.Identify.Intents = discordgo.IntentsAll

	if err := discordSess.Open(); err != nil {
		LogError(err, "Error while opening Discord session")
		return
	}

	// Unregister commands before shutting down
	LogInfo("Bot shutting down, unregistering commands")
	unregisterCommands(discordSess)
}

func unregisterCommands(s *discordgo.Session) {
	guilds, err := s.UserGuilds(100, "", "", false)
	if err != nil {
		LogError(err, "Error getting guilds during unregister")
		return
	}

	// Get all registered commands for each guild and delete them
	for _, guild := range guilds {
		registeredCommands, err := s.ApplicationCommands(s.State.User.ID, guild.ID)
		if err != nil {
			LogError(err, "Error getting commands for guild")

			continue
		}
		for _, cmd := range registeredCommands {
			if err := s.ApplicationCommandDelete(s.State.User.ID, guild.ID, cmd.ID); err != nil {
				LogError(err, "Error deleting command in guild")

			}
		}
	}

	// Also unregister global commands
	globalCommands, err := s.ApplicationCommands(s.State.User.ID, "")
	if err != nil {
		LogError(err, "Error getting global commands")
		return
	}
	for _, cmd := range globalCommands {
		if err := s.ApplicationCommandDelete(s.State.User.ID, "", cmd.ID); err != nil {
			LogError(err, "Error deleting global command")
		}
	}

	LogInfo("Discord commands unregistered successfully")
}

func registerCommands(s *discordgo.Session) {
	// First, get all guilds the bot is in
	guilds, err := s.UserGuilds(100, "", "", false)
	if err != nil {
		LogError(err, "Error getting guilds")

	}

	// Register commands to each guild for faster updates during development
	for _, guild := range guilds {
		LogInfo("Registering commands to guild")

		for _, command := range commands {
			if _, err := s.ApplicationCommandCreate(s.State.User.ID, guild.ID, command); err != nil {
				LogError(err, "Error creating command in guild")

			}
		}
	}

	// Also register globally as a backup, but these take up to an hour to propagate
	for _, command := range commands {
		if _, err := s.ApplicationCommandCreate(s.State.User.ID, "", command); err != nil {
			LogError(err, "Error creating global command")
		}
	}

	LogInfo("Slash commands registered successfully")
}

func onReady(discordSess *discordgo.Session, event *discordgo.Ready) {
	LogInfo("Bot is ready")

	ChannelID := "998632857306136617"
	messages, _ := discordSess.ChannelMessages(ChannelID, 100, "", "", "")

	slices.Reverse(messages)
	for _, msg := range messages {
		fmt.Printf("%s : %s\n", msg.Author.Username, msg.Content)
	}
}

func printMessage(ctx context.Context, discordSess *discordgo.Session, message *discordgo.MessageCreate) {
	channel, err := discordSess.Channel(message.ChannelID)
	if err != nil {
		LogError(err, "Could not retrieve channel")
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

	// Create context for this message processing
	ctx := context.Background()
	printMessage(ctx, discordSess, message)
}

func interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Create a context for tracing this interaction
	ctx := context.Background()

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
		handleLLMCommand(ctx, s, i, options)
	case "upload":
		handleUploadCommand(ctx, s, i)
	case "stats":
		handleStatsCommand(ctx, s, i)
	case "help":
		handleHelpCommand(ctx, s, i)
	}
}

func handleHelpCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	helpText := `Commands:
- /wis llm [text] - Large Language Model
- /wis upload - Upload a file
- /wis help - Show this message
- /wis stats - Show some stats about the server`

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: helpText},
	})

	if err != nil {
		LogError(err, "Failed to respond to help command")
	}
}

func handleLLMCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	// Acknowledge the interaction to prevent timeout
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	if err != nil {
		LogError(err, "Failed to acknowledge LLM command")
		return
	}

	text := options[0].StringValue()

	// Previous chat message collection removed

	// username := getUserFromInteraction(i)
	LogInfo("Sending prompt to LLM")

	// Use single message channel instead of chat channel
	InputChannel <- text
	response := <-OutputChannel

	LogInfo("Received LLM response")

	chunks, err := chunkDiscordMessage(response, 1995)
	if err != nil {
		LogError(err, "Error chunking message")

		str := fmt.Sprintf("Error: %v", err)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &str,
		})
		return
	}

	// Send the first chunk as the interaction response
	_, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &chunks[0],
	})

	if err != nil {
		LogError(err, "Failed to edit response")
	}

	// Send additional chunks as follow-up messages
	for _, chunk := range chunks[1:] {
		_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: chunk,
		})

		if err != nil {
			LogError(err, "Failed to send follow-up message")
		}

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

func handleUploadCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	db, err := GetDBQueries()

	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Upload feature is unavailable (database service error): " + err.Error(),
			},
		})
		return
	}

	uuid := uuid.New()
	username := getUserFromInteraction(i)

	// Count the number of files the user has uploaded
	count, err := db.CountFilesFromUser(ctx, username)
	if err != nil {
		LogError(err, "Failed to count user files")

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
		err := db.DeleteFileWhereUsersCountIsProvided(ctx,
			sqlc.DeleteFileWhereUsersCountIsProvidedParams{
				DiscordUsername: username,
				Limit:           int32(count - maxFilesPerUser + 1),
			})
		if err != nil {
			LogError(err, "Failed to delete old files")

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Error cleaning up old files: %v", err),
				},
			})
			return
		}

		LogInfo("Deleted old files to stay under limit")
	}

	// Insert the new file
	err = db.InsertFile(ctx, sqlc.InsertFileParams{
		ID:              uuid.String(),
		Uploaded:        false,
		DiscordUsername: username,
		Name:            "empty file",
	})

	if err != nil {
		LogError(err, "Failed to insert file")

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Error creating file entry: %v", err),
			},
		})
		return
	}

	LogInfo("Created new file entry")

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Here is the link: https://%s/id/%s", httpServerIp, uuid.String()),
		},
	})
}

func handleStatsCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	guild, err := s.Guild(i.GuildID)

	if err != nil {
		LogError(err, "Failed to get guild info")

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

	LogInfo("Retrieved stats")

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: stats},
	})
}
