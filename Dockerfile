FROM golang:1.22-alpine AS builder

ARG TAG
ENV TAG ${TAG}

# Move to working directory /build
WORKDIR /build

# Install git
RUN apk add --no-cache git

# Clone repo
RUN if [ -z "${TAG}" ]; then \
      git clone --depth 1 https://github.com/Elegant996/rtorrent-healthz .; \
    else \
      git clone -b v${TAG} --depth 1 https://github.com/Elegant996/rtorrent-healthz .; \
    fi

# Download dependency using go mod
RUN go mod download

# Build the application
RUN go build -o healthz .

# Build a small image
FROM scratch

COPY --from=builder /build/healthz /

# Command to run
ENTRYPOINT ["/healthz"]