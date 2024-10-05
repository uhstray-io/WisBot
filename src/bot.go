package main

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

func StartBot() {
	fmt.Println("Starting bot...")
	discord, err := discordgo.New("Bot " + config.DiscordToken)
	if err != nil {
		print("Error encountered while creating Discord session. ", err)
		return
	}
	defer discord.Close()

	// Register the messageCreate func as a callback for MessageCreate events.
	discord.AddHandler(messageCreate)
	// discord.AddHandler(onReady)

	// In this example, we only care about receiving message events.
	discord.Identify.Intents = discordgo.IntentsAll

	// Open a websocket connection to Discord and begin listening.
	err = discord.Open()
	if err != nil {
		fmt.Println("Error: encountered while opening connection.", err)
		return
	}
}

func onReady(sess *discordgo.Session, event *discordgo.Ready) {
	fmt.Printf("Logged in as: %v#%v \n", sess.State.User.Username, sess.State.User.Discriminator)

	// ServerID := "889910011113906186"
	ChannelID := "998632857306136617"

	message, _ := sess.ChannelMessages(ChannelID, 100, "", "", "")

	slices.Reverse(message)
	for _, msg := range message {
		fmt.Printf("%s : %s\n", msg.Author.Username, msg.Content)
	}
}

func printMessage(sess *discordgo.Session, message *discordgo.MessageCreate) {
	messageTime := message.Timestamp.Local().Format("01-02-2006 15:04:05")

	// Dummy guild for private messages
	guild := &discordgo.Guild{Name: "Private Message"}

	channel, err := sess.Channel(message.ChannelID)
	if err != nil {
		fmt.Println("Error: encountered while getting channel.", err)
		return
	}

	// If the channel is a private message, set the name to "Private Message"
	if channel.Type == discordgo.ChannelTypeDM {
		channel.Name = "Private Message"
	} else {
		guild, err = sess.Guild(message.GuildID)
		if err != nil {
			fmt.Println("Error: encountered while getting guild.", err)
			return
		}
	}

	println(fmt.Sprintf("%s (%s) %s", guild.ID, channel.ID, messageTime))
	println(fmt.Sprintf("%s - %s\n", message.Author.Username, message.Content))
}

type MessageProperties struct {
	IsPrivate bool
	IsCommand bool

	ReplyChannelID string
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func messageCreate(sess *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Author.ID == sess.State.User.ID {
		return
	}

	printMessage(sess, message)

	fmt.Println("GuildID", message.GuildID)
	fmt.Println("Type", message.Type)
	fmt.Println("ChannelID", message.ChannelID)
	fmt.Println("ID", message.ID)
	fmt.Println("Author", message.Author)
	fmt.Println("Content", message.Content)

	fmt.Println("message.Author.Username", message.Author.Username)
	fmt.Println("message.Author.Discriminator", message.Author.Discriminator)
	fmt.Println("message.Author.ID", message.Author.ID)
	fmt.Println("message.Author.GlobalName", message.Author.GlobalName)
	fmt.Println("message.Author.String()", message.Author.String())

	messageProperties := &MessageProperties{
		IsPrivate: false,
		IsCommand: false,

		ReplyChannelID: message.ChannelID,
	}

	// Simple Commands
	if strings.HasPrefix(message.Content, "/wis") {
		message.Content = strings.TrimSpace(strings.TrimPrefix(message.Content, "/wis"))
		messageProperties.IsCommand = true
	}

	if strings.HasPrefix(message.Content, "?") {
		message.Content = strings.TrimSpace(strings.TrimPrefix(message.Content, "?"))
		messageProperties.IsPrivate = true

		userChannel, _ := sess.UserChannelCreate(message.Author.ID)
		messageProperties.ReplyChannelID = userChannel.ID
	}

	parseCommand(sess, message, messageProperties)
}

func parseCommand(sess *discordgo.Session, message *discordgo.MessageCreate, messageProperties *MessageProperties) {

	if !messageProperties.IsCommand {
		return
	}

	if message.Content == "upload" {
		uploadCommand(message, sess)
		return
	}

	if strings.HasPrefix(message.Content, "llm") {
		llmCommand(message, messageProperties, sess)
		return
	}

	if messageProperties.IsPrivate {
		mess := fmt.Sprintf("Hello %s, this is private!", message.Author.Username)
		sess.ChannelMessageSend(messageProperties.ReplyChannelID, mess)
	} else {
		mess := fmt.Sprintf("Hello %s", message.Author.Username)
		sess.ChannelMessageSend(messageProperties.ReplyChannelID, mess)
	}

	if message.Content == "ping" {
		sess.ChannelMessageSend(message.ChannelID, "Pong!")
	}

	if message.Content == "pong" {
		sess.ChannelMessageSend(message.ChannelID, "Ping!")
	}
}

func llmCommand(message *discordgo.MessageCreate, messageProperties *MessageProperties, sess *discordgo.Session) {
	input := strings.TrimSpace(strings.TrimPrefix(message.Content, "llm"))

	InputChannel <- input
	output := <-OutputChannel

	mess := fmt.Sprintf("LLM: %s", output)

	chunks := chunkDiscordMessage(mess, 1995)
	for _, chunk := range chunks {
		sess.ChannelMessageSend(messageProperties.ReplyChannelID, chunk)
		time.Sleep(200 * time.Millisecond)
	}
}

func uploadCommand(message *discordgo.MessageCreate, sess *discordgo.Session) {
	uuid := uuid.New()

	// Count the number of files the user has uploaded.
	var count int
	err3 := db.QueryRow("SELECT COUNT(*) FROM File WHERE DiscordUsername = ?", message.Author.Username).Scan(&count)
	if err3 != nil {
		fmt.Println("Error counting files.", err3.Error())
	}

	// Remove the oldest files if the user has uploaded too many.
	if count >= config.MaxFilesPerUser {
		_, err1 := db.Exec("DELETE FROM File WHERE ROWID IN ( SELECT ROWID FROM File WHERE DiscordUsername = ? ORDER BY CreatedAt LIMIT ?)", message.Author.Username, count-config.MaxFilesPerUser+1)
		if err1 != nil {
			fmt.Println("Error removing old files.", err1.Error())
		}
	}

	// Insert the new file.
	_, err2 := db.Exec("INSERT INTO File (ID, Uploaded, DiscordUsername, Name) VALUES (?, ?, ?, ?)", uuid.String(), false, message.Author.Username, "empty file")
	if err2 != nil {
		fmt.Println("Error when adding UUID to DB.", err2.Error())
	}

	mess := fmt.Sprintf("Here is the link: http://%s:%s/id/%s", config.Server.IpAddr, config.Server.Port, uuid.String())
	sess.ChannelMessageSend(message.ChannelID, mess)
}
