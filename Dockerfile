FROM golang:1.23.2

WORKDIR /app

# Install ollama
RUN curl -fsSL https://ollama.com/install.sh | sh

# Copy the current directory contents into the container at /app
COPY . .

RUN go build -o main ./src

# Configure container
ENV PORT=8080

EXPOSE 8080

# Start app 
CMD ["./main"]