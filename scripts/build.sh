#!/bin/bash

templ generate
sqlc generate -f ./src/sqlc/sqlc.yaml

go build -o ./tmp/main.exe ./src/