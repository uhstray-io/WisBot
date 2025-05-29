#!/bin/bash

templ generate
sqlc generate -f ./src/sqlc/sqlc.yaml

docker compose -f ./deployment/compose.yaml down

docker build -t wisbot -f ./deployment/Dockerfile .

docker compose -f ./deployment/compose.yaml up -d
