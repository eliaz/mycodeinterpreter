# Use a specific version of the Golang image for consistent builds
FROM golang:1.20 AS builder

# Set the working directory in the build stage
WORKDIR /app

# Copy go.mod and go.sum files first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o mycodeinterpreter .

# Use a smaller base image for the runtime
FROM ubuntu:latest  
RUN apt-get update && apt-get install -y ca-certificates sudo curl wget golang-go python3 python3-pip && rm -rf /var/lib/apt/lists/*
WORKDIR /root/

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/mycodeinterpreter .

# Set environment variables
# Replace with actual values or remove if not needed
ENV NGROK_AUTHTOKEN=your_default_ngrok_token
ENV AUTH_KEY=noauth
#ENV SAFE_MODE="-nosafe"
#ENV SEMI_SAFE="-semisafe"

# Run the Go program when the container launches
CMD ./mycodeinterpreter "${AUTH_KEY}" "${SAFE_MODE}" "${SEMI_SAFE}"

