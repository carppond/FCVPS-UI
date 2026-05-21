# deploy/Dockerfile — Hub multi-stage build
# Stage 1 (node-build): compile frontend → web/dist
# Stage 2 (go-build):   compile Go binary with embedded frontend
# Stage 3 (runtime):    distroless static image < 30 MB
#
# Build:
#   docker buildx build --file deploy/Dockerfile \
#     --platform linux/amd64,linux/arm64 \
#     --tag shiguang-vps:latest .

# ---------------------------------------------------------------------------
# Stage 1: Frontend build
# ---------------------------------------------------------------------------
FROM node:20-alpine AS node-build

WORKDIR /src/web

# Install pnpm
RUN corepack enable && corepack prepare pnpm@latest --activate

# Cache node_modules layer separately from source
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

# Copy the rest of the frontend sources and build
COPY web/ ./
RUN pnpm build

# ---------------------------------------------------------------------------
# Stage 2: Go build
# ---------------------------------------------------------------------------
FROM golang:1.25-alpine AS go-build

# Install git for module downloads; ca-certificates for TLS
RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Download modules first (Docker layer cache)
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source tree
COPY . .

# Overwrite web/dist with the freshly-built frontend (embed.FS picks this up)
COPY --from=node-build /src/web/dist ./web/dist

# Build the hub binary.  CGO_ENABLED=0 required for distroless static.
RUN CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w" \
      -trimpath \
      -o /out/shiguang-vps \
      ./cmd/server

# ---------------------------------------------------------------------------
# Stage 3: Runtime — distroless/static (no shell, no libc, minimal attack surface)
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

# Binary
COPY --from=go-build /out/shiguang-vps /shiguang-vps

# Timezone data (optional, harmless to include)
COPY --from=go-build /usr/share/zoneinfo /usr/share/zoneinfo

# /data is the persistent-state mount point (SQLite + uploads)
# The nonroot user (uid=65532) must be able to write here at runtime.
# Operators should mount a host directory with matching permissions.
VOLUME ["/data"]

EXPOSE 8080

ENTRYPOINT ["/shiguang-vps"]
