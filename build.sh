#!/bin/bash

templ generate
sqlc generate -f ./src/sql/sqlc.yaml

go build -o ./tmp/main.exe ./src/