
# WisBot

A bot for the automation of things

## Diagrams
![diagram](./diagrams/userflow.excalidraw.png)


## Commands

`/wis help` - Shows the help message

`/wis upload` - Uploads a file to the server


## Requirements
- Golang 1.23
- Discord Token
- Ollama 
- Llama3.2 model `ollama pull llama3.2`


## Running 
You can run the bot using the following command:
```sh
go run ./src
```

## Tooling
After the installation of Go, the following tools are recommended for development. Please install them using the commands below:

### Templ
A language for writing HTML user interfaces in Go - https://github.com/a-h/templ
```sh
go install github.com/a-h/templ/cmd/templ@latest
```

### Air
Live reload for Go apps - https://github.com/air-verse/air
```sh
go install github.com/air-verse/air@latest
```


## Running the bot
> [!NOTE]
You will need a config.yaml file, if you don't have one, one will be created for you on the first run.
Please fill out the config. This is for Discord authentication. You can get the token from the Discord Developer Portal.



## Building the Docker Image
```sh
docker build -t wisbot .
```