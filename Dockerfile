# syntax=docker/dockerfile:1

# --- frontend build stage ---
FROM node:22-alpine AS node-build
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# --- Go build stage ---
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=node-build /src/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /chattermax ./cmd/server

# --- runtime stage ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget && adduser -D -u 10001 app
COPY --from=build /chattermax /usr/local/bin/chattermax
USER app
ENV PORT=8080
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null 2>&1 || exit 1
ENTRYPOINT ["/usr/local/bin/chattermax"]
