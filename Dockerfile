FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod ./
COPY cmd/action ./cmd/action
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" -o /out/action-entrypoint ./cmd/action

FROM alpine:3.23 AS selene

ARG SELENE_VERSION=0.30.1
ARG SELENE_SHA256=c8c0f2102cb37e5e3ee2c984b51946b8ea8cf7804b5ea067afdb42fd2b95ff6e
ARG SELENE_LIGHT_SHA256=a1a4a5a458fd1a3ea9335b8b6446c6abe731306c41a2b6f28adcf1835e25f6d5

RUN apk add --no-cache ca-certificates curl unzip \
    && mkdir -p /out \
    && curl --fail --location --silent --show-error \
        --output /tmp/selene.zip \
        "https://github.com/Kampfkarren/selene/releases/download/${SELENE_VERSION}/selene-${SELENE_VERSION}-linux.zip" \
    && echo "${SELENE_SHA256}  /tmp/selene.zip" | sha256sum --check --strict \
    && unzip -p /tmp/selene.zip selene > /out/selene \
    && curl --fail --location --silent --show-error \
        --output /tmp/selene-light.zip \
        "https://github.com/Kampfkarren/selene/releases/download/${SELENE_VERSION}/selene-light-${SELENE_VERSION}-linux.zip" \
    && echo "${SELENE_LIGHT_SHA256}  /tmp/selene-light.zip" | sha256sum --check --strict \
    && unzip -p /tmp/selene-light.zip selene > /out/selene-light \
    && chmod 0755 /out/selene /out/selene-light

FROM gcr.io/distroless/cc-debian12:nonroot

COPY --from=selene /out/selene /usr/local/bin/selene
COPY --from=selene /out/selene-light /usr/local/bin/selene-light
COPY --from=builder /out/action-entrypoint /action-entrypoint

ENTRYPOINT ["/action-entrypoint"]
