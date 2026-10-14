# Multi-stage Dockerfile for Life Dashboard Hub

# Stage 1: Build SolidJS Frontend
FROM node:22-alpine AS frontend-builder
WORKDIR /web
COPY web/package.json ./
RUN npm install
COPY web/ ./
RUN npm run build

# Stage 2: Build Go Server
FROM golang:1.27-alpine AS backend-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
# Copy frontend build output into embed path
COPY --from=frontend-builder /web/dist ./cmd/server/dist
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build -tags embed_frontend \
    -ldflags="-s -w \
    -X github.com/awang-karisma/life-dashboard-hub/internal/version.Version=${VERSION} \
    -X github.com/awang-karisma/life-dashboard-hub/internal/version.Commit=${COMMIT} \
    -X github.com/awang-karisma/life-dashboard-hub/internal/version.Date=${DATE}" \
    -o /app/life-dashboard-hub ./cmd/server

# Stage 3: Minimal runtime image
FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

COPY --from=backend-builder /app/life-dashboard-hub /app/life-dashboard-hub

ENV DATA_DIR=/data \
    LISTEN_ADDR=:8080

VOLUME ["/data"]
EXPOSE 8080

ENTRYPOINT ["/app/life-dashboard-hub"]
