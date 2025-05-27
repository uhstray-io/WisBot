#!/bin/bash

templ generate
sqlc generate -f ./src/sql/sqlc.yaml

docker build -t wisbot .

if [ "$1" = "prod" ]; then
  # Use the production compose file
  docker compose -f compose.yaml up -d
else
  # Use the development compose file
  docker compose -f compose.dev.yaml up -d
fi