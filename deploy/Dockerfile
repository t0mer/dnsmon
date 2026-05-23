# Stage 1: Build Tailwind CSS
FROM node:20-alpine AS frontend
WORKDIR /app
COPY web/package*.json web/
RUN cd web && npm ci
COPY web/ web/
RUN cd web && npx tailwindcss -i src/css/tailwind.src.css -o dist/css/app.css --minify && \
    mkdir -p dist/js && cp src/js/*.js dist/js/ && cp src/*.html dist/

# Stage 2: Build Go binary
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/dist web/dist
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w \
      -X github.com/tomerklein/gdns/internal/version.Version=${VERSION} \
      -X github.com/tomerklein/gdns/internal/version.Commit=${COMMIT} \
      -X github.com/tomerklein/gdns/internal/version.Date=${DATE}" \
    -o /gdns ./cmd/gdns

# Stage 3: Minimal runtime image
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /gdns /gdns
EXPOSE 8080
ENTRYPOINT ["/gdns"]
