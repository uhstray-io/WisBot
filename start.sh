echo "DISCORD_TOKEN_WISBOT=$DISCORD_TOKEN_WISBOT"

docker compose down
docker build -t wisbot .
docker compose up -d
