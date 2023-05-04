FROM golang:1.20-alpine AS builder

ARG VERSION

# Set necessary environment variables needed for our image
# ENV GO111MODULE=on \
#     CGO_ENABLED=0 \
#     GOOS=linux \
#     GOARCH=amd64

# Move to working directory /build
WORKDIR /build

# Install git
RUN apk add --no-cache git

# Clone repo
RUN if [ -z "${VERSION}" ]; then \
      git clone --depth 1 https://github.com/Elegant996/rtorrent-healthz .; \
    else \
      git clone -b v${VERSION} --depth 1 https://github.com/Elegant996/rtorrent-healthz .; \
    fi

# Download dependency using go mod
RUN go mod download

# Build the application
RUN go build -o main .

# Move to /dist directory as the place for resulting binary folder
WORKDIR /dist

# Copy binary from build to main folder
RUN cp /build/main .

# Build a small image
FROM scratch

COPY --from=builder /dist/main /

# Command to run
ENTRYPOINT ["/main"]