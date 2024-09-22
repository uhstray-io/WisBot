package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"gopkg.in/yaml.v3"

	_ "github.com/mattn/go-sqlite3"
	_ "modernc.org/sqlite"
)

var config *Config

type Config struct {
	IpAddr string `yaml:"ip"`
	Port   string `yaml:"port"`

	DiscordToken string `yaml:"discord_token"`
	DataBaseName string `yaml:"database_name"`
}

func LoadConfig(filename string) {
	// This function will fail if the file doesn't exist.
	file, err := os.Open(filename)

	c := &Config{}

	// If the file doesn't exist, create it and write the default config to it
	if err != nil {
		fmt.Println("Config file not found, creating a new one.")

		// Create a new config and close the program
		file_new, err := os.Create(filename)
		if err != nil {
			fmt.Println("Error creating config file", err.Error())
		}
		defer file_new.Close()

		c.IpAddr = "127.0.0.1"
		c.Port = "8080"
		c.DiscordToken = "YOUR_DISCORD_TOKEN"
		c.DataBaseName = "database.db"

		// Write the default config to the file
		err = yaml.NewEncoder(file_new).Encode(c)
		if err != nil {
			fmt.Println("Error writing default config to file", err.Error())
		}
		os.Exit(1)
	}
	defer file.Close()

	err = yaml.NewDecoder(file).Decode(c)

	if err != nil {
		fmt.Println("Error decoding config file", err.Error())
		os.Exit(1)
	}

	config = c
}

func main() {
	// Load configuration
	LoadConfig("config.yaml")

	// Initialize the database
	db, err := initDatabase()
	if err != nil {
		fmt.Println("Error initializing database:", err.Error())
		os.Exit(1)
	}
	defer db.Close()

	go StartBot()
	go WebServer()

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
