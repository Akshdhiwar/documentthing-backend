# Stage 1: Build the Go binary
FROM golang:1.22-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Copy .env file into the container
COPY .env .env

RUN go build -o /app/bin/main ./cmd/app

# Stage 2: Run the Go binary in a smaller image
FROM alpine:latest

WORKDIR /app

COPY --from=build /app/bin/main .
COPY --from=build /app/.env .

EXPOSE 8080

CMD ["./main"]
