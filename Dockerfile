# syntax=docker/dockerfile:1
#
# Two images, two languages, one wire contract:
#
#   docker build --target relay -t github-activity-feed/relay .
#   docker build --target feed  -t github-activity-feed/feed  .
#
# docker-compose.yml builds both. Why these versions and how to bump them:
# docs/docker.md.

# Patched releases, not floating majors. Both lines ship security releases on a
# schedule, so the version here is the date of the last one (docs/docker.md).
ARG NODE_VERSION=24.22.1
ARG GO_VERSION=1.27.3
ARG ALPINE_VERSION=3.24.3

# ---------------------------------------------------------------------------
# relay: the NestJS image. Two stages, because the toolchain that compiles
# TypeScript and the native SQLite binding has no business in the runtime.
# ---------------------------------------------------------------------------
FROM node:${NODE_VERSION}-bookworm-slim AS relay-build
WORKDIR /app

# better-sqlite3 is a native addon. The compiler chain is here so npm can build
# it when no prebuilt binary matches this platform and ABI.
RUN apt-get update \
 && apt-get install -y --no-install-recommends python3 make g++ \
 && rm -rf /var/lib/apt/lists/*

COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

COPY tsconfig.json tsconfig.build.json nest-cli.json ./
COPY src ./src
RUN npm run build && npm prune --omit=dev

FROM node:${NODE_VERSION}-bookworm-slim AS relay
ENV NODE_ENV=production
WORKDIR /app

COPY --from=relay-build /app/node_modules ./node_modules
COPY --from=relay-build /app/dist ./dist
COPY --from=relay-build /app/package.json ./package.json

# The SQLite file lives on a volume: without one, every restart forgets the
# history the CLI exists to read.
RUN mkdir -p /data && chown -R node:node /data
ENV DATABASE_PATH=/data/relay.db

USER node
EXPOSE 3000
VOLUME ["/data"]

# A relay without a webhook secret refuses to boot, so a healthy container is
# one that answered /history: it proves the environment is complete and the
# database opened.
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD node -e "fetch('http://127.0.0.1:'+(process.env.PORT||3000)+'/history').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"

CMD ["node", "dist/main.js"]

# ---------------------------------------------------------------------------
# feed: the Go client. The toolchain is pinned because a Go security release is
# a standard-library fix, and the binary carries the standard library it was
# built with. CGO off is what makes the runtime stage small: a static binary
# needs no libc.
# ---------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS feed-build
ARG TARGETOS TARGETARCH
ARG FEED_VERSION=dev
WORKDIR /src

COPY client/ ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -buildvcs=false \
      -ldflags="-s -w -X main.version=${FEED_VERSION}" \
      -o /out/feed ./cmd/feed

FROM alpine:${ALPINE_VERSION} AS feed
# ca-certificates: the CLI can point at an https:// relay, and a static binary
# brings no trust store of its own.
RUN apk add --no-cache ca-certificates \
 && adduser -D -H -u 10001 -s /sbin/nologin feed

COPY --from=feed-build /out/feed /usr/local/bin/feed

USER feed
ENTRYPOINT ["feed"]
CMD ["--version"]
