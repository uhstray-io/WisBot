
# WisBot

A bot for the automation of things

## Commands

`/wis help` - Shows the help message

`/wis upload` - Uploads a file to the server


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
You will need to create a token.key file in the root of the project with the token for the bot to work. This is for Discord authentication. You can get the token from the Discord Developer Portal.