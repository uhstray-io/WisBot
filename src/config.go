package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

var config *Config

type Config struct {
	DiscordToken string `yaml:"discord_token"`
}

func LoadConfig(filename string) {
	// This function will fail if the file doesn't exist.
	file, err := os.Open(filename)

	localConfig := &Config{}

	// If the file doesn't exist, create it and write the default config to it
	if err != nil {
		fmt.Println("Config file not found, creating a new one.")

		// Create a new config and close the program
		file_new, err := os.Create(filename)
		if err != nil {
			fmt.Println("Error creating config file", err.Error())
		}
		defer file_new.Close()

		localConfig.DiscordToken = "YOUR_DISCORD_TOKEN"

		// Write the default config to the file
		err = yaml.NewEncoder(file_new).Encode(localConfig)
		if err != nil {
			fmt.Println("Error writing default config to file", err.Error())
		}
		os.Exit(1)
	}
	defer file.Close()

	err = yaml.NewDecoder(file).Decode(localConfig)

	if err != nil {
		fmt.Println("Error decoding config file", err.Error())
		os.Exit(1)
	}

	config = localConfig
}
