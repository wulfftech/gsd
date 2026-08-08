# Stage 1: Build the frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /ui

COPY ui/package.json ui/package-lock.json ./
RUN npm ci --force @rollup/rollup-linux-x64-gnu @swc/core-linux-x64-gnu

COPY ui/ ./
RUN npx vite build --mode selfhosted

# Stage 2: Build the Go binary
FROM golang:1.24-alpine AS go-builder

WORKDIR /app

ARG VERSION=dev
ARG COMMIT=dev
ARG BUILD_DATE=dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Overwrite frontend/dist with the freshly built frontend
COPY --from=frontend-builder /ui/dist ./frontend/dist

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildDate=${BUILD_DATE}" -o gsd .

# Stage 3: Minimal runtime image
FROM alpine:3.20

RUN apk --no-cache add ca-certificates libc6-compat tzdata

# Create non-root user
RUN addgroup -S app && adduser -S app -G app

COPY --from=go-builder /app/gsd /gsd
COPY --from=go-builder /app/config /config

# Ensure app user owns directories it needs to access
RUN chmod 755 /config && chown -R app:app /config

ENV DT_ENV="selfhosted"

EXPOSE 2021

USER app

CMD ["/gsd"]
