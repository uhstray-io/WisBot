package main

import (
	"context"
	"fmt"
	"slices"
	"time"
	"wisbot/src/sqlgo"

	"go.opentelemetry.io/otel/log"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
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

func StartBot(ctx context.Context) {
	ctx, span := StartSpan(ctx, "bot.StartBot")
	defer span.End()

	LogEvent(ctx, log.SeverityInfo, "Starting Discord bot")

	discordSess, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		err = fmt.Errorf("error while creating Discord session: %w", err)
		ErrorTrace(err)
	}
	defer discordSess.Close()

	discordSess.AddHandler(messageCreate)
	discordSess.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		fmt.Println("Bot is ready! Logged in as:", s.State.User.Username)
		// Create a new context for the handler since we don't have the original
		handlerCtx, handlerSpan := StartSpan(ctx, "bot.onReady")
		defer handlerSpan.End()

		LogEvent(handlerCtx, log.SeverityInfo, "Bot ready", attribute.String("username", s.State.User.Username))
		registerCommands(handlerCtx, s)
	})
	discordSess.AddHandler(interactionCreate)

	discordSess.Identify.Intents = discordgo.IntentsAll

	err2 := discordSess.Open()
	if err2 != nil {
		err2 = fmt.Errorf("error while opening Discord session: %w", err2)
		ErrorTrace(err2)
	}

	// Block until context is done
	<-ctx.Done()

	// Unregister commands before shutting down
	cleanupCtx, cleanupSpan := StartSpan(ctx, "bot.cleanup")
	defer cleanupSpan.End()

	LogEvent(cleanupCtx, log.SeverityInfo, "Bot shutting down, unregistering commands")
	unregisterCommands(cleanupCtx, discordSess)
}

func unregisterCommands(ctx context.Context, s *discordgo.Session) {
	ctx, span := StartSpan(ctx, "bot.unregisterCommands")
	defer span.End()

	// Get all guilds the bot is in
	guilds, err := s.UserGuilds(100, "", "", false)
	if err != nil {
		fmt.Printf("Error getting guilds during unregister: %v", err)
		span.RecordError(err)
		return
	}

	// Get all registered commands for each guild and delete them
	for _, guild := range guilds {
		registeredCommands, err := s.ApplicationCommands(s.State.User.ID, guild.ID)
		if err != nil {
			fmt.Printf("Error getting commands for guild %s: %v", guild.ID, err)
			span.RecordError(err)
			continue
		}

		for _, cmd := range registeredCommands {
			err := s.ApplicationCommandDelete(s.State.User.ID, guild.ID, cmd.ID)
			if err != nil {
				fmt.Printf("Error deleting command %s in guild %s: %v", cmd.Name, guild.ID, err)
				span.RecordError(err)
			}
		}
	}

	// Also unregister global commands
	globalCommands, err := s.ApplicationCommands(s.State.User.ID, "")
	if err != nil {
		fmt.Printf("Error getting global commands: %v", err)
		span.RecordError(err)
		return
	}

	for _, cmd := range globalCommands {
		err := s.ApplicationCommandDelete(s.State.User.ID, "", cmd.ID)
		if err != nil {
			fmt.Printf("Error deleting global command %s: %v", cmd.Name, err)
			span.RecordError(err)
		}
	}

	LogEvent(ctx, log.SeverityInfo, "Discord commands unregistered successfully")
}

func registerCommands(ctx context.Context, s *discordgo.Session) {
	ctx, span := StartSpan(ctx, "bot.registerCommands")
	defer span.End()

	// First, get all guilds the bot is in
	guilds, err := s.UserGuilds(100, "", "", false)
	if err != nil {
		fmt.Printf("Error getting guilds: %v", err)
		span.RecordError(err)
	}

	// Register commands to each guild for faster updates during development
	for _, guild := range guilds {
		fmt.Printf("Registering commands to guild: %s (%s)", guild.Name, guild.ID)
		for _, command := range commands {
			_, err := s.ApplicationCommandCreate(s.State.User.ID, guild.ID, command)
			if err != nil {
				fmt.Printf("Error creating command %v in guild %s: %v", command.Name, guild.ID, err)
				span.RecordError(err)
			}
		}
	}

	// Also register globally as a backup, but these take up to an hour to propagate
	for _, command := range commands {
		_, err := s.ApplicationCommandCreate(s.State.User.ID, "", command)
		if err != nil {
			fmt.Printf("Error creating global command %v: %v", command.Name, err)
			span.RecordError(err)
		}
	}

	LogEvent(ctx, log.SeverityInfo, "Slash commands registered successfully")
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
	// Create a context for tracing this interaction
	ctx, span := StartSpan(context.Background(), "bot.interactionCreate")
	defer span.End()

	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := i.ApplicationCommandData()
	span.SetAttributes(attribute.String("command", data.Name))

	if data.Name != "wis" {
		return
	}

	// Get the subcommand
	subcommand := data.Options[0].Name
	span.SetAttributes(attribute.String("subcommand", subcommand))
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
	ctx, span := StartSpan(ctx, "bot.handleHelpCommand")
	defer span.End()

	helpText := `Commands:
- /wis llm [text] - Large Language Model
- /wis upload - Upload a file
- /wis help - Show this message
- /wis stats - Show some stats about the server`

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: helpText,
		},
	})

	if err != nil {
		span.RecordError(err)
		LogEvent(ctx, log.SeverityError, "Failed to respond to help command", attribute.String("error", err.Error()))
	}
}

func handleLLMCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	ctx, span := StartSpan(ctx, "bot.handleLLMCommand")
	defer span.End()

	// Acknowledge the interaction to prevent timeout
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	if err != nil {
		span.RecordError(err)
		LogError(ctx, err, "Failed to acknowledge LLM command")
		return
	}

	text := options[0].StringValue()
	span.SetAttributes(attribute.String("prompt", text))

	chatMessages, err := s.ChannelMessages(i.ChannelID, 100, "", "", "")
	if err != nil {
		span.RecordError(err)
		LogError(ctx, err, "Failed to fetch channel messages")
	}

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

	LogEvent(ctx, log.SeverityInfo, "Sending prompt to LLM",
		attribute.String("user", getUserFromInteraction(i)),
		attribute.Int("context_messages", len(UserMessages)))

	InputChatChannel <- UserMessages
	response := <-OutputChatChannel

	LogEvent(ctx, log.SeverityInfo, "Received LLM response", attribute.Int("response_length", len(response)))

	chunks, err := chunkDiscordMessage(response, 1995)
	if err != nil {
		span.RecordError(err)
		LogError(ctx, err, "Error chunking message")

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
		span.RecordError(err)
		LogError(ctx, err, "Failed to edit response")
	}

	// Send additional chunks as follow-up messages
	for _, chunk := range chunks[1:] {
		_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: chunk,
		})

		if err != nil {
			span.RecordError(err)
			LogError(ctx, err, "Failed to send follow-up message")
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
	ctx, span := StartSpan(ctx, "bot.handleUploadCommand")
	defer span.End()

	uuid := uuid.New()
	username := getUserFromInteraction(i)

	span.SetAttributes(attribute.String("user", username), attribute.String("file_id", uuid.String()))

	// Count the number of files the user has uploaded
	count, err := wisQueries.CountFilesFromUser(ctx, username)
	if err != nil {
		span.RecordError(err)
		LogError(ctx, err, "Failed to count user files",
			attribute.String("user", username),
			attribute.String("error", err.Error()))

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
		err := wisQueries.DeleteFileWhereUsersCountIsProvided(ctx,
			sqlgo.DeleteFileWhereUsersCountIsProvidedParams{
				DiscordUsername: username,
				Limit:           int32(count - maxFilesPerUser + 1),
			})
		if err != nil {
			span.RecordError(err)
			LogError(ctx, err, "Failed to delete old files",
				attribute.String("user", username),
				attribute.String("error", err.Error()))

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Error cleaning up old files: %v", err),
				},
			})
			return
		}

		LogError(ctx, err, "Deleted old files to stay under limit",
			attribute.String("user", username),
			attribute.Int64("deleted", count-maxFilesPerUser+1))
	}

	// Insert the new file
	err = wisQueries.InsertFile(ctx, sqlgo.InsertFileParams{
		ID:              uuid.String(),
		Uploaded:        false,
		DiscordUsername: username,
		Name:            "empty file",
	})
	if err != nil {
		span.RecordError(err)
		LogError(ctx, err, "Failed to insert file",
			attribute.String("file_id", uuid.String()),
			attribute.String("error", err.Error()))

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Error creating file entry: %v", err),
			},
		})
		return
	}

	LogError(ctx, err, "Created new file",
		attribute.String("user", username),
		attribute.String("file_id", uuid.String()))

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Here is the link: https://%s/id/%s", serverIp, uuid.String()),
		},
	})
}

func handleStatsCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx, span := StartSpan(ctx, "bot.handleStatsCommand")
	defer span.End()

	guild, err := s.Guild(i.GuildID)
	if err != nil {
		span.RecordError(err)
		LogError(ctx, err, "Failed to get guild info", attribute.String("error", err.Error()))

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

	LogError(ctx, err, "Retrieved stats",
		attribute.Int("members", memberCount),
		attribute.Int("servers", serverCount),
		attribute.Int("channels", channelCount))

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: stats,
		},
	})
}
