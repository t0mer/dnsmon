# syntax=docker/dockerfile:1

# Stage 1: build the embedded frontend assets (Tailwind CSS + copied HTML/JS).
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: cross-compile the Go binary for the target platform.
FROM --platform=$BUILDPLATFORM golang:1.25.10-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Bring in the built frontend (overwrites web/dist so //go:embed dist sees fresh assets).
COPY --from=frontend /app/web/dist web/dist

ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} \
    go build -trimpath \
      -ldflags="-s -w -X github.com/t0mer/dnsmon/internal/version.Version=${VERSION}" \
      -o /dnsmon ./cmd/dnsmon

# Stage 3: minimal distroless runtime.
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /dnsmon /dnsmon
EXPOSE 8080
ENTRYPOINT ["/dnsmon"]
