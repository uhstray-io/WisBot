# Stage 1
FROM golang:1.23.2 AS builder

WORKDIR /app

# Install Templ for generating html
RUN go install github.com/a-h/templ/cmd/templ@latest

# Copy only required files
COPY go.sum go.mod ./
COPY config.yaml ./
COPY /src ./src

# Run Templ
RUN templ generate

# Build the Go app with cache
RUN --mount=type=cache,target=/root/.cache/go-build \
--mount=type=cache,target=/go/pkg/mod \
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
go build -o main ./src

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
COPY --from=builder /app/config.yaml ./

# Start app 
CMD ["./main"]