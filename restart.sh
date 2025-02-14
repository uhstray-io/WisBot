#!/bin/bash

templ generate
sqlc generate -f ./src/sql/sqlc.yaml

docker compose down
docker build -t wisbot .
docker compose up -d
