FROM golang:1.24-alpine AS builder

COPY . ./

# Download dependencies
RUN go mod download

# Build the application
RUN go build -o /dist/healthz ./src

# Build a small image
FROM scratch

COPY --from=builder /dist/healthz /

# Command to run
ENTRYPOINT ["/healthz"]