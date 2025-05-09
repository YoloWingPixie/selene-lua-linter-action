# ---- Builder Stage ----
FROM golang:tip-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY . .

# Build the Go application statically
# CGO_ENABLED=0 for static linking, ldflags to reduce binary size.
RUN echo "Building Go application..." && \
    CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /action-entrypoint ./cmd/action/main.go && \
    echo "Go application built successfully." && \
    ls -l /action-entrypoint

# ---- Selene Fetcher Stage ----
FROM alpine:latest AS selene-fetcher

RUN apk add --no-cache \
    curl \
    jq \
    unzip

ARG SELENE_REPO=Kampfkarren/selene
ARG SELENE_VARIANT=selene # Standard variant, not selene-light for preloading
WORKDIR /tmp/selene-download

# Download and extract Selene using the existing script logic (simplified)
RUN echo "Fetching latest Selene release from ${SELENE_REPO} for pre-installation..." && \
    LATEST_RELEASE_ASSET_INFO=$(curl -fsSL "https://api.github.com/repos/${SELENE_REPO}/releases/latest" | jq --arg variant "$SELENE_VARIANT" -r '.assets[] | select(.name | test("^" + $variant + "(-[0-9.]+)?-linux\\.zip$")) | {name, url: .browser_download_url} | @json') && \
    if [ -z "$LATEST_RELEASE_ASSET_INFO" ] || [ "$LATEST_RELEASE_ASSET_INFO" == "null" ]; then \
        echo "Error: Could not find a suitable Selene release asset." >&2; \
        exit 1; \
    fi && \
    LATEST_RELEASE_URL=$(echo "$LATEST_RELEASE_ASSET_INFO" | jq -r .url) && \
    LATEST_RELEASE_FILENAME=$(echo "$LATEST_RELEASE_ASSET_INFO" | jq -r .name) && \
    echo "Found asset: $LATEST_RELEASE_FILENAME" && \
    echo "Downloading from: $LATEST_RELEASE_URL" && \
    curl -fsSL -o selene_latest.zip "$LATEST_RELEASE_URL" && \
    unzip -o selene_latest.zip -d /tmp/selene_extracted && \
    SELENE_BINARY_PATH=$(find /tmp/selene_extracted -name "$SELENE_VARIANT" -type f | head -n 1) && \
    if [ -z "$SELENE_BINARY_PATH" ]; then \
        echo "Error: Selene binary not found after extraction." >&2; \
        exit 1; \
    fi && \
    mv "$SELENE_BINARY_PATH" /usr/local/bin/selene && \
    chmod +x /usr/local/bin/selene && \
    echo "Selene binary from release moved to /usr/local/bin/selene" && \
    rm selene_latest.zip && \
    rm -rf /tmp/selene_extracted && \
    echo "Cleaned up temporary files."

# ---- Runner Stage ----
FROM gcr.io/distroless/cc-debian12 AS runner

# Copy CA certificates from a stage that has them (selene-fetcher is alpine, so it has them)
# Distroless images often need these explicitly for HTTPS.
COPY --from=selene-fetcher /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=selene-fetcher /usr/local/bin/selene /usr/local/bin/selene

COPY --from=builder /action-entrypoint /action-entrypoint

USER nonroot:nonroot

ENTRYPOINT ["/action-entrypoint"]