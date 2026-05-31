# Stage 1: Build the frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /ui

COPY ui/package.json ./
RUN npm install --force @rollup/rollup-linux-x64-gnu @swc/core-linux-x64-gnu

COPY ui/ ./
RUN npx vite build --mode selfhosted

# Stage 2: Build the Go binary
FROM golang:1.24-alpine AS go-builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Overwrite frontend/dist with the freshly built frontend
COPY --from=frontend-builder /ui/dist ./frontend/dist

RUN CGO_ENABLED=0 GOOS=linux go build -o gsd .

# Stage 3: Minimal runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates libc6-compat tzdata

COPY --from=go-builder /app/gsd /gsd
COPY --from=go-builder /app/config /config

ENV DT_ENV="selfhosted"

EXPOSE 2021

CMD ["/gsd"]
