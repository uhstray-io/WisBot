# Stage 1
FROM golang:1.24.4 AS builder

WORKDIR /app

# Install Templ for generating html
RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

# Copy only required files
COPY go.sum go.mod ./
RUN go mod download

COPY src ./src

COPY .env dev.env ./

# Run Templ
RUN go tool templ generate
RUN sqlc generate -f ./src/sqlc/sqlc.yaml

# Build the Go app with cache
RUN --mount=type=cache,target=/root/.cache/go-build \
--mount=type=cache,target=/go/pkg/mod \
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
go build -o main ./src

# Create a placeholder dev.env if it doesn't exist (to ensure COPY always succeeds)  
RUN if [ ! -f dev.env ]; then touch dev.env; fi

# Append dev.env to .env if it has content
RUN touch .env && \
    if [ -s dev.env ]; then \
        cat dev.env >> .env; \
    fi && \
    rm dev.env

# NOTE: the mount=type=cache flags are used to cache the go modules and go build files between builds and drastically reduce the build time

# NOTE: the GOOS=linux GOARCH=amd64 CGO_ENABLED=0 flags are required to build a statically linked binary for alpine

# Stage 2
FROM alpine:latest AS final

# We are using a final image of alpine as it is a very lightweight image.

WORKDIR /app

# Configure container
ENV PORT=8080
EXPOSE 8080

# Copy executable to final container
COPY --from=builder /app/main ./
COPY --from=builder /app/.env ./

# Start app 
CMD ["./main"]