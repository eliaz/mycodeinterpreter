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
FROM ubuntu:22.04
RUN apt-get update
RUN apt-get install -y ca-certificates sudo curl wget golang-go python3 python3-pip vim
RUN rm -rf /var/lib/apt/lists/*
RUN ln -s /usr/bin/python3 /usr/bin/python

WORKDIR /root/

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/mycodeinterpreter .

# Declare arguments for build-time configuration
ARG NGROK_AUTHTOKEN=your_default_ngrok_token
ARG AUTH_KEY=noauth
ARG SAFE_MODE="-nosafe"
ARG SEMI_SAFE="-semisafe"

# Set environment variables using ARG values
ENV NGROK_AUTHTOKEN=${NGROK_AUTHTOKEN}
ENV AUTH_KEY=${AUTH_KEY}
ENV SAFE_MODE=${SAFE_MODE}
ENV SEMI_SAFE=${SEMI_SAFE}

# Run the Go program when the container launches
CMD ./mycodeinterpreter "${AUTH_KEY}" "${SAFE_MODE}" "${SEMI_SAFE}"
