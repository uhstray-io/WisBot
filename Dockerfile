FROM golang:1.23.2

WORKDIR /app

# Install ollama
# RUN curl -fsSL https://ollama.com/install.sh | sh
# RUN ollama serve & sleep 5 && ollama pull llama3.2

# Configure container
ENV PORT=8080

EXPOSE 8080

# Copy the current directory contents into the container at /app
COPY . .

# RUN go build -o main ./src
RUN --mount=type=cache,target=/root/.cache/go-build \
--mount=type=cache,target=/go/pkg/mod \
go build -o main ./src

# Start app 
CMD ["./main"]