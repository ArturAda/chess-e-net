FROM node:20-alpine AS frontend-builder

WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi
COPY frontend ./
RUN npm run build

FROM golang:1.26-alpine AS backend-builder

WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /src/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/chess-server ./cmd/server

FROM alpine:3.22 AS runtime

WORKDIR /app
RUN apk add --no-cache ca-certificates \
    && addgroup -S chess \
    && adduser -S chess -G chess

COPY --from=backend-builder /out/chess-server /app/chess-server
COPY --from=frontend-builder /src/frontend/dist /app/frontend/dist
COPY README.md README.ru.md /app/
COPY LICENSE /app/LICENSE

ENV PORT=8080
ENV GIN_MODE=release
ENV FRONTEND_DIST_DIR=/app/frontend/dist

EXPOSE 8080
USER chess

CMD ["/app/chess-server"]
