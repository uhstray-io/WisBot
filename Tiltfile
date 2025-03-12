# -*- mode: Python -*-

# For more on Extensions, see: https://docs.tilt.dev/extensions.html
load('ext://restart_process', 'docker_build_with_restart')
load('ext://docker_compose', 'docker_compose')

# Allow Tilt to use Docker Compose for orchestration
docker_compose('compose.yaml')

# Generate Templ files
local_resource(
    'templ-generate',
    'templ generate',
    deps=['./src/**/*.templ'],
    labels=['codegen']
)

# Generate SQLC files
local_resource(
    'sqlc-generate',
    'sqlc generate -f ./src/sql/sqlc.yaml',
    deps=['./src/sql/query/**/*.sql', './src/sql/schema/**/*.sql', './src/sql/sqlc.yaml'],
    labels=['codegen']
)

# Go build command based on OS
go_build_cmd = 'GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ./tmp/main ./src/'
if os.name == 'nt':
    go_build_cmd = 'go build -o ./tmp/main.exe ./src/'

# Compile the Go application
local_resource(
    'wisbot-compile',
    go_build_cmd,
    deps=['./src/**/*.go'],
    resource_deps=['templ-generate', 'sqlc-generate'],
    labels=['build']
)

# Build Docker image with live reload capability
docker_build_with_restart(
    'wisbot',
    '.',
    entrypoint=['./main'],
    dockerfile='Dockerfile',
    live_update=[
        sync('./src', '/app/src'),
        sync('./tmp/main', '/app/main'),
        # Run code generators inside container when templates change
        run('cd /app && templ generate', trigger=['./src/**/*.templ']),
        run('cd /app && sqlc generate -f ./src/sql/sqlc.yaml', 
            trigger=['./src/sql/query/**/*.sql', './src/sql/schema/**/*.sql'])
    ],
    match_in_env_vars=True
)

# Configure resource settings for the Docker Compose services
dc_resource('wisbot', 
    labels=['app'],
    resource_deps=['wisbot-compile']
)

dc_resource('db', 
    labels=['infra']
)

dc_resource('llm', 
    labels=['infra']
)

dc_resource('db_dashboard', 
    labels=['tools'],
    port_forwards='1234:8080'
)
