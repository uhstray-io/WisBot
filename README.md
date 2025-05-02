<!-- omit in toc -->
# WisBot

<!-- omit in toc -->
## A Multi-functional Discord Automation Bot

WisBot is a versatile automation bot that combines Discord integration, AI capabilities, file management, and a web interface into one powerful solution.

**Key Features:**
- **Discord Integration:** Seamless connection with Discord API for command handling and message processing
- **AI Capabilities:** Integration with Ollama to provide LLM functionality for natural language interactions
- **File Management:** Robust file upload/download system with database storage, user tracking, and automatic cleanup
- **Web Interface:** User-friendly web interface built with Go templates for interaction outside Discord
- **Observability:** Comprehensive monitoring and tracing with OpenTelemetry integration

---

- [Getting Started](#getting-started)
  - [Quick Start Guide](#quick-start-guide)
  - [Tiltfile Quickstart](#tiltfile-quickstart)
- [Architecture](#architecture)
  - [WisBot Design Architecture](#wisbot-design-architecture)
  - [User Experience Workflows](#user-experience-workflows)
- [Commands](#commands)
- [Core Beliefs](#core-beliefs)
- [Requirements \& Dependencies](#requirements--dependencies)
  - [Runtime Requirements](#runtime-requirements)
  - [Development Requirements](#development-requirements)
    - [Templ, SQLc \& Air](#templ-sqlc--air)
- [Running the bot](#running-the-bot)
  - [Running the bot using Go](#running-the-bot-using-go)
- [Prepare Linux (Ubuntu 22.04) for running Wisbot](#prepare-linux-ubuntu-2204-for-running-wisbot)
- [Setup Nvidia Drivers and Container Toolkit on Ubuntu 22.04](#setup-nvidia-drivers-and-container-toolkit-on-ubuntu-2204)
- [Using Docker to build and deploy Wisbot](#using-docker-to-build-and-deploy-wisbot)
  - [Building the Docker Image](#building-the-docker-image)
  - [Running the bot using docker-compose](#running-the-bot-using-docker-compose)
    - [Running the dockerfile with GPU acceleration enabled:](#running-the-dockerfile-with-gpu-acceleration-enabled)
- [Ollama Docker Image Documentation](#ollama-docker-image-documentation)
  - [Environment Variables](#environment-variables)
  - [Web Interface](#web-interface)
  - [Issues](#issues)
- [CRLF vs LF](#crlf-vs-lf)

## Getting Started

### Quick Start Guide

1. Set up your environment variables (see [Running the bot](#running-the-bot))
2. Deploy using Docker or Go (see [deployment options](#running-the-bot))
3. Interact with WisBot using [commands](#commands)

- [Review our Code of Conduct](https://www.uhstray.io/en/code-of-conduct)
- [Check our CONTRIBUTING.MD](./CONTRIBUTING.md)

### Tiltfile Quickstart

The `Tiltfile` in this repository is designed to streamline the development process for WisBot. It automates the setup of the development environment, including:

1. Uses Docker Compose to orchestrate the services defined in compose.yaml
2. Sets up code generation for Templ and SQLC, watching for changes in template and SQL files
3. Compiles the Go application with proper settings for Linux targets
4. Configures Docker build with live updates to sync code changes without full rebuilds
5. Sets resource dependencies to ensure proper build order
6. Organizes resources with labels for better UI grouping
7. Configures port forwarding for the database dashboard

With this configuration, developers can run `tilt up` to start WisBot with live reload capabilities, making the development workflow much more efficient.

## Architecture

### WisBot Design Architecture

```mermaid
flowchart TD
subgraph subgraph1["Users"]
    User["Discord Members"]
    WebUser["Web Users"]
end
subgraph subGraph5["Connected Services"]
    NocoDB["NocoDB"]
    Postiz["Postiz Social Media Scheduler"]
    Superset["Superset Data Analytics"]
end
subgraph WisBot["WisBot"]
    Discord["Discord API"]
    WebSite["WebSite"]
    DiscordBot["Discord Bot"]
    FileManager["File Manager"]
    WisLLM["WisLLM Processor"]
    WebServerAPI["Web Server REST API"]
end
subgraph subGraph3["LLM PLatform"]
    Models[("Model Storage")]
    Ollama["Ollama LLM Service"]
end
subgraph DataBases["Data Warehousing"]
    subgraph subGraph4["Data Cache and Pipelines"]
        Redis["Redis Cache"]
    end
        DataWarehouse[("Data Warehouse PostgreSQL DB")]
        WisbotDB[("Wisbot PostgreSQL DB")]
        bizS3["minIO S3 Object Storage"]
        Drives["Documents/Drives - Proton, Sharepoint, Google"]
        VectorDB[("pgVector Database")]
end





    Ollama -- "llama3.2" --> Models
    User --> Discord
    WebUser --> WebSite
    WebSite --> WebServerAPI
    Discord --> DiscordBot
    DiscordBot --> WisLLM & FileManager
    WebServerAPI --> WisLLM & FileManager
    FileManager --> WisbotDB
    WisbotDB <--> DataWarehouse
    DataWarehouse <--> VectorDB
    VectorDB <--> Drives
    WisLLM <--> WisbotDB
    bizS3 <--> VectorDB
    Postiz --> WebServerAPI
    WisLLM <--> Ollama
    Postiz --> Redis --> bizS3
    NocoDB --> bizS3 & DataWarehouse & WebServerAPI
    Superset --> DataWarehouse & bizS3
```

### User Experience Workflows

![User Experience Diagrams](./diagrams/userflow.excalidraw.png)

## Commands

| Command | Description |
|---------|-------------|
| `/wis help` | Displays available commands and usage information |
| `/wis upload` | Uploads a file to the server for processing or storage |
| `/wis llm <text>` | Sends a request to the integrated LLM and returns its response |
| `/wis stats` | Displays system statistics and performance metrics |

## Core Beliefs

WisBot adheres to these core principles:

- **Reliability**
  - May never fail silently
  - Must always trace errors with complete stack information
  - Must return meaningful errors to users
  - Must fail gracefully when external systems are unavailable

- **Performance**
  - Must be fast and responsive
  - Must handle resources efficiently
  - Must enforce user quotas and limits
  - Must cleanup expired resources automatically

- **Architecture**
  - Must maintain separation of concerns
  - Must be containerized and deployable
  - Must be observable and monitorable
  - Must scale when needed

- **User Experience**
  - Must communicate clearly with users
  - Must keep data secure

## Requirements & Dependencies

### Runtime Requirements
- Discord Token
- PostgreSQL database
- Ollama (optional, for LLM functionality)
- Llama3.2 model if using Ollama (`ollama pull llama3.2`)
- Nvidia GPU with Container Toolkit (optional, for improved LLM performance)
- Docker and Docker Compose (recommended deployment method)

### Development Requirements
- Golang 1.24
- Git
- The following development tools:

#### Templ, SQLc & Air

- **Templ**: A language for writing HTML user interfaces in Go - https://github.com/a-h/templ
- **Air**: Live reload for Go apps - https://github.com/air-verse/air
- **SQLc**: SQL compiler for Go - https://github.com/sqlc-dev/sqlc
  
```sh
go install github.com/a-h/templ/cmd/templ@latest && go install github.com/air-verse/air@latest && go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

Run the following command to generate the Templ files for the bot:
```sh
templ generate
```

Run the following command to generate the SQLC files for the bot:
```sh
sqlc generate -f ./src/sql/sqlc.yaml
```

Then run air in the WisBot repository to update all required dependencies:
```sh
air
```

## Running the bot
> [!NOTE]
You will need a .env file with the following environment variables:
```
# WISBOT ENV VARIABLES
SERVER_IP=wisbot.yourdomain.com
SERVER_PORT=8080

# WISBOT CONFIG
MAX_FILES_PER_USER=3
DELETE_FILES_AFTER_DAYS=7
MAX_FILE_SIZE=250 # in MB
DATABASE_URL=postgres://username:password@localhost:5432/database_name

# OLLAMA ENV VARIABLES
OLLAMA_URL=http://10.5.0.3:11434
OLLAMA_MODEL=llama3.2
OLLAMA_KEEP_ALIVE=24h

# POSTGRES ENV VARIABLES
POSTGRES_USER=username
POSTGRES_PASSWORD=password
POSTGRES_DB=database_name
POSTGRES_PORT=5432

# DISCORD TOKEN
DISCORD_TOKEN_WISBOT=your_discord_token
```

### Running the bot using Go

You can run the bot using the following command:
```sh
go run ./src
```

## Prepare Linux (Ubuntu 22.04) for running Wisbot

Remove any existing docker packages and conflicting dependencies:
```sh
for pkg in docker.io docker-doc docker-compose docker-compose-v2 podman-docker containerd runc; do sudo apt-get remove $pkg; done
```

Setup docker-apt repository:

```sh
# Add Docker's official GPG key:
sudo apt-get update
sudo apt-get install ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

# Add the repository to Apt sources:
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update
```

Install docker:

```sh
sudo apt-get install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
```

Add the user to the docker group:

```sh
sudo usermod -aG docker $USER
```

Install Github Runner on Ubuntu 22.04

https://github.com/uhstray-io/WisBot/settings/actions/runners/new?arch=x64&os=linux

```sh
mkdir actions-runner && cd actions-runner
```

Download the latest runner package:

```sh
curl -o actions-runner-linux-x64-2.320.0.tar.gz -L https://github.com/actions/runner/releases/download/v2.320.0/actions-runner-linux-x64-2.320.0.tar.gz
```

Validate the Runner:

```sh
echo "93ac1b7ce743ee85b5d386f5c1787385ef07b3d7c728ff66ce0d3813d5f46900  actions-runner-linux-x64-2.320.0.tar.gz" | shasum -a 256 -c
```

Extract the runner:

```sh
tar xzf ./actions-runner-linux-x64-2.320.0.tar.gz
```

Configure the runner:

```sh
./config.sh --url <MY_URL> --token <MY_TOKEN>
```

Test the runner:

```sh
./run.sh
```

Setup the github runner as a service:

https://docs.github.com/en/actions/hosting-your-own-runners/managing-self-hosted-runners/configuring-the-self-hosted-runner-application-as-a-service

```sh
sudo ./svc.sh install
```

Start the github runner service:

```sh
sudo ./svc.sh start
```

Restart docker and enable the BuildKit to ensure compatibility with the deployment:

```sh
DOCKER_BUILDKIT=1
sudo systemctl restart docker
```

You can use the `stop` and `uninstall` commands to stop and uninstall the service.

## Setup Nvidia Drivers and Container Toolkit on Ubuntu 22.04

https://developer.nvidia.com/datacenter-driver-downloads?target_os=Linux&target_arch=x86_64&Distribution=Ubuntu&target_version=22.04&target_type=deb_network

Setup the Nvidia drivers GPG key:
```sh
wget https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2204/x86_64/cuda-keyring_1.1-1_all.deb
sudo dpkg -i cuda-keyring_1.1-1_all.deb
sudo apt-get update
```

Install the OpenKernel Drivers:
```sh
sudo apt-get install -y nvidia-open-565
```

Add the container toolkit repository:
```sh
curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg \
  && curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
    sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
    sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
```

Configure Experimental Packages:
```sh
sudo sed -i -e '/experimental/ s/^#//g' /etc/apt/sources.list.d/nvidia-container-toolkit.list
```

Update the repository and install the Nvidia Container Toolkit:

```sh
sudo apt-get update && sudo apt-get install -y nvidia-container-toolkit
```

Configure the toolkit to use Docker:
```sh
sudo nvidia-ctk runtime configure --runtime=docker
```

Restart Docker:
```sh
sudo systemctl restart docker
```

## Using Docker to build and deploy Wisbot

### Building the Docker Image

Update the latest build of the wisbot:

```sh
docker build -t wisbot .
```

Running the dockerfile via Docker:

[docker run](https://docs.docker.com/reference/cli/docker/container/run/)

```sh
docker run -d wisbot
```

### Running the bot using docker-compose

[docker compose](https://docs.docker.com/compose/)

```sh
docker compose up -d
```

```sh
docker compose down
```

Quick scripts for starting, stopping, and restarting the application:

```sh
# Start
./start.sh

# Stop
./stop.sh  

# Restart (down, rebuild, up)
./restart.sh
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

https://docs.docker.com/desktop/gpu/

## Ollama Docker Image Documentation
https://hub.docker.com/r/ollama/ollama

### Environment Variables

WisBot uses the following environment variables for Ollama:

- `OLLAMA_URL`: The URL where the Ollama service is running (e.g., http://10.5.0.3:11434)
- `OLLAMA_MODEL`: The model to use (e.g., llama3.2)
- `OLLAMA_KEEP_ALIVE`: How long to keep the model loaded (e.g., 24h)

### Web Interface

The compose file includes configuration for an optional Ollama web UI, which is currently commented out. Uncomment the `ollama-webui` service in the compose.yaml file if you want to use it.

### Issues
## CRLF vs LF

```sh
git config core.eol lf
git config core.autocrlf input
```
