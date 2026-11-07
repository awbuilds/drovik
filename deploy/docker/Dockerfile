# syntax=docker/dockerfile:1.7

# ---- Frontend ---------------------------------------------------------------
FROM node:22-alpine AS frontend
WORKDIR /app
RUN npm install -g pnpm@12.4.1
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
RUN pnpm build

# ---- Backend ----------------------------------------------------------------
FROM golang:1.27 AS backend
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
FROM alpine:3.22 AS dirs
RUN mkdir -p /data

FROM gcr.io/distroless/static-debian12:nonroot
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
