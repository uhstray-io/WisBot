# COPILOT INSTRUCTIONS
## EDITS OPERATIONAL GUIDELINES

Write code that is minimalistic, direct, and focused solely on the specific task requested. 
Prioritize clarity and simplicity. 
ALWAYS start by creating a detailed plan BEFORE making any edits
Only implement exactly what is asked, without adding unnecessary comments, extra functionality, or speculative improvements. 
Respond with precise, lean code that addresses the core requirements only. 
Please keep the structure generally similar so that we can see the changes via git diff.
Always consider libraries installed in the go.mod file when implementing new functionality.

## Tools and Commands 
When trying to compile the code, run the build.sh script


## Folder Structure
Follow this structured directory layout:
project-root/
├── .github/
│   └── copilot-instructions.md
├── src/
│   ├── sqlc/
│   │   ├── db.go
│   │   ├── models.go
│   │   ├── query.sql
│   │   ├── query.sql.go
│   │   ├── schema.sql
│   │   └── schema.sql.go
│   ├── templ/
│   │   ├── *.go
│   │   ├── *.templ
│   │   └── 
│   ├── database.go
│   ├── discord.go
│   ├── httpserver.go
│   ├── fileupload.go
|   ├── llm.go
│   ├── main.go
│   └── *.go
├── deployment/
│   ├── config/
│   │   ├── rules.yaml
│   │   ├── prometheus.yaml
│   │   ├── otel-collect-config.yaml
│   ├── entrypoints/
│   │   ├── ollama.sh
│   ├── compose.yaml
│   ├── compose.dev.yaml
│   ├── Dockerfile
├── scripts/
│   ├── build.sh
│   ├── start.sh
│   ├── restart.sh
│   └── stop.sh
└── README.md