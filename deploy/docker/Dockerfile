# syntax=docker/dockerfile:1.7

# ---- Frontend ---------------------------------------------------------------
FROM node:22-alpine@sha256:0e1754758c2df4cfa3e68d59e2e1b7374bb7fd9ea6328eaaacf15cc934af8202 AS frontend
WORKDIR /app
# Corepack ships with Node and verifies the pnpm package integrity from the
# registry; no unpinned global npm command is used.
RUN corepack enable && corepack prepare pnpm@12.4.1 --activate
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
RUN pnpm build

# ---- Backend ----------------------------------------------------------------
FROM golang:1.27@sha256:e519d3b0a5b285e5aa53ab9eef02bc6bbb9cb04608389b743a136a9ce75f0d1e AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w \
      -X github.com/awbuilds/drovik/backend/internal/version.Version=${VERSION} \
      -X github.com/awbuilds/drovik/backend/internal/version.Commit=${COMMIT} \
      -X github.com/awbuilds/drovik/backend/internal/version.BuildDate=${BUILD_DATE}" \
    -o /out/drovik ./cmd/drovik

# ---- Runtime ----------------------------------------------------------------
FROM alpine:3.22@sha256:bfe7ceb9f09ec0566c48fb93356e45a47a00be5a1dd14fa5b909e7709c66b034 AS dirs
RUN mkdir -p /data

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
WORKDIR /app

COPY --from=backend /out/drovik /drovik
COPY --from=frontend /app/dist /app/public
COPY --from=dirs --chown=65532:65532 /data /data

ENV DROVIK_ENV=production \
    DROVIK_HTTP_ADDR=0.0.0.0:8080 \
    DROVIK_DATA_DIR=/data \
    DROVIK_DB_PATH=/data/drovik.db \
    DROVIK_STATIC_DIR=/app/public \
    DROVIK_LOG_FORMAT=json

EXPOSE 8080
VOLUME ["/data"]

USER nonroot:nonroot
ENTRYPOINT ["/drovik"]
