# Multi-stage Dockerfile for building a small, reproducible Go app
# Stage 1: Build
FROM golang:1.21-alpine AS builder

# Install git for go modules that reference VCS and ca-certificates for fetching modules
RUN apk add --no-cache git ca-certificates

# Create app user/group for the final image (uid/gid are chosen deterministically)
ARG BUILD_UID=1000
ARG BUILD_GID=1000

WORKDIR /src

# Copy go.mod and go.sum first to leverage Docker layer caching for deps
COPY go.mod go.sum ./

# Use a module-aware build to download dependencies
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go env -w GOPATH=/go && \
    go env -w GOCACHE=/root/.cache/go-build && \
    go mod download

# Copy the rest of the source
COPY . .

# Build a statically-linked, optimized binary
ARG APP_NAME=docksleek
ARG BUILD_TIME="unknown"
ARG VCS_REF="none"
ARG VERSION="dev"

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags "-s -w -X main.buildTime=${BUILD_TIME} -X main.vcsRef=${VCS_REF} -X main.version=${VERSION}" -o /out/${APP_NAME} ./


# Stage 2: Minimal runtime image

FROM gcr.io/distroless/static:nonroot AS runtime

# Copy CA certs for TLS & the built binary from the builder stage
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/docksleek /usr/local/bin/docksleek

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/docksleek","-check-health"]

ENTRYPOINT ["/usr/local/bin/docksleek"]

LABEL org.opencontainers.image.title="Docksleek"
LABEL org.opencontainers.image.description="Lightweight image for running the Docksleek Go linter/utility"
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.source="https://github.com/AyanJhunjhunwala/Docksleek"
