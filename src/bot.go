package main

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

func readToken() (string, error) {
	// Read fileData from file
	fileData, err := os.ReadFile("token.key")
	if err != nil {
		return "", errors.New("error encountered while reading token file. please make sure that token.key exists in the same directory as the executable")
	}

	token := string(fileData)
	return token, nil
}

func StartBot() {
	token, err := readToken()
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Starting bot...")
	discord, err := discordgo.New("Bot " + token)
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
	}

	// If the channel is a private message, set the name to "Private Message"
	if channel.Type == discordgo.ChannelTypeDM {
		channel.Name = "Private Message"
	} else {
		guild, err = sess.Guild(message.GuildID)
		if err != nil {
			fmt.Println("Error: encountered while getting guild.", err)
		}
	}

	res1 := fmt.Sprintf("%s (%s) %s", guild.ID, channel.ID, messageTime)
	res2 := fmt.Sprintf("%s - %s\n", message.Author.Username, message.Content)
	println(res1)
	println(res2)
}

type messageProperties struct {
	IsPrivate bool
	IsCommand bool
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

	messageProperties := &messageProperties{}

	// Simple Commands
	if strings.HasPrefix(message.Content, "/wis") {
		message.Content = strings.TrimSpace(strings.TrimPrefix(message.Content, "/wis"))
		messageProperties.IsCommand = true
	}

	if strings.HasPrefix(message.Content, "?") {
		message.Content = strings.TrimSpace(strings.TrimPrefix(message.Content, "?"))
		messageProperties.IsPrivate = true
	}

	parseCommand(sess, message, messageProperties)
}

func parseCommand(sess *discordgo.Session, message *discordgo.MessageCreate, messageProperties *messageProperties) {

	if !messageProperties.IsCommand {
		return
	}

	if message.Content == "upload" {
		uuid := uuid.New()

		_, err := db.Exec("INSERT INTO File (ID, Uploaded, DiscordUsername, Name) VALUES (?, ?, ?, ?)", uuid.String(), false, message.Author.Username, "")

		if err != nil {
			fmt.Println("Error in parseCommand", err)
		}

		mess := fmt.Sprintf("Here is the link: http://"+URL+":8000/id/%s", uuid.String())
		sess.ChannelMessageSend(message.ChannelID, mess)
	}

	if messageProperties.IsPrivate {
		userChannel, _ := sess.UserChannelCreate(message.Author.ID)

		mess := fmt.Sprintf("Hello %s, this is private!", message.Author.Username)
		sess.ChannelMessageSend(userChannel.ID, mess)
	} else {
		mess := fmt.Sprintf("Hello %s", message.Author.Username)
		sess.ChannelMessageSend(message.ChannelID, mess)
	}

	if message.Content == "ping" {
		sess.ChannelMessageSend(message.ChannelID, "Pong!")
	}

	if message.Content == "pong" {
		sess.ChannelMessageSend(message.ChannelID, "Ping!")
	}
}
