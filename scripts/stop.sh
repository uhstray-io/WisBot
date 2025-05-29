#!/bin/bash

# Bring down all running containers
docker compose -f ./deployment/compose.yaml down --remove-orphans
