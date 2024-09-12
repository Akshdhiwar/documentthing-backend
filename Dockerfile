# Use Go 1.22 with the Alpine Linux distribution as the base image
FROM golang:1.22-alpine

# Set the working directory inside the container
WORKDIR /app

# Install any needed packages (like git, build-base) to build the application
RUN apk add --no-cache git

# Copy the Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the Go app
RUN go build -o app cmd/app/main.go

# Expose port (if needed, replace 8080 with your application's port)
EXPOSE 8080

# Run the Go app
CMD ["./app"]
