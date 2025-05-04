FROM golang:1.24-alpine AS builder

COPY . ./src

# Build the application
RUN go build -o dist/healthz .

# Build a small image
FROM scratch

COPY --from=builder /dist/healthz /

# Command to run
ENTRYPOINT ["/healthz"]