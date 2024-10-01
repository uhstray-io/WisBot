package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

var config *Config

type Config struct {
	Server struct {
		IpAddr string `yaml:"ip"`
		Port   string `yaml:"port"`
	} `yaml:"server"`

	Database struct {
		Type string `yaml:"type"`
		Name string `yaml:"name"`
	} `yaml:"database"`

	DiscordToken         string `yaml:"discord_token"`
	MaxFilesPerUser      int    `yaml:"max_files_per_user"`
	DeleteFilesAfterDays int    `yaml:"delete_files_after_days"`

	LLM struct {
		Name string `yaml:"name"`
	} `yaml:"llm"`
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

		localConfig.Server.IpAddr = "127.0.0.1"
		localConfig.Server.Port = "8080"
		localConfig.Database.Type = "sqlite"
		localConfig.Database.Name = "database.db"
		localConfig.DiscordToken = "YOUR_DISCORD_TOKEN"
		localConfig.MaxFilesPerUser = 3
		localConfig.DeleteFilesAfterDays = 7

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
