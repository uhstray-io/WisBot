
# WisBot

A bot for the automation of things

## Diagrams

![diagram](./diagrams/userflow.excalidraw.png)


## Commands

`/wis help` - Shows the help message

`/wis upload` - Uploads a file to the server

`/wis llm` - Sends a request to the attached WisBot LLM


## Requirements
- Golang 1.23
- Templ (optional)
- Discord Token
- Ollama 
- Llama3.2 model `ollama pull llama3.2`
- Nvidia Container Toolkit (optional)

### Install Templ dependency manager

```sh
go install github.com/a-h/templ/cmd/templ@latest
```

Generate the necessary Go dependencies
```sh
templ generate
```

## Running and building the bot

### Running the bot using Go

You can run the bot using the following command:
```sh
go run ./src
```

### Running the bot using docker and a Dockerfile

Update the latest build of the wisbot:
```sh
docker build -t wisbot .
```

Running the dockerfile via Docker:

[docker run](https://docs.docker.com/reference/cli/docker/container/run/)

```sh
docker run -d wisbot
```

#### Running the dockerfile with GPU acceleration enabled:

Enable Nvidia Container Toolkit resources on Ubuntu 22.04 WSL:

[nvidia container toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html#prerequisites)

```sh
curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg && curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
```

Update the package list and install the Nvidia Container Toolkit:

```sh
sudo apt-get update && sudo apt-get install -y nvidia-container-toolkit
```

Configure Docker to use the nvidia-container-toolkit:

```sh
sudo nvidia-ctk runtime configure --runtime=docker && sudo systemctl restart docker
```

Run the image with GPU acceleration:

```sh
docker run -d wisbot --gpus all ubuntu nvidia-smi
```

### Running the bot using docker-compose

[docker compose](https://docs.docker.com/compose/)

```sh
docker compose up -d
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