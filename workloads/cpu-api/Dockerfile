ARG BASE_IMAGE=alpine:latest

# Stage 1: Compile the Go binary (always uses the golang image)
FROM golang:alpine AS builder
WORKDIR /app
COPY go.mod main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o server .

# Stage 2: Runtime image (swappable via build arg)
FROM ${BASE_IMAGE}
COPY --from=builder /app/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
