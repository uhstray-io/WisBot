#!/bin/bash

templ generate
sqlc generate -f ./src/sqlc/sqlc.yaml

docker build -t wisbot -f ./deployment/Dockerfile .

if [ "$1" = "prod" ]; then
  # Use the production compose file
  docker compose -f ./deployment/compose.yaml up -d
else
  # Use the development compose file
  docker compose -f ./deployment/compose.dev.yaml up -d
fi