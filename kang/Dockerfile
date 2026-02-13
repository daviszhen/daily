# Stage 1: Build frontend
FROM node:20-alpine AS frontend
WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build
RUN cp mo-logo.png /build/server/cmd/server/dist/

# Stage 2: Build backend
FROM golang:1.23-alpine AS backend
ENV GOPRIVATE=github.com/matrixorigin
WORKDIR /build/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
COPY --from=frontend /build/server/cmd/server/dist ./cmd/server/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /smart-daily ./cmd/server/

# Stage 3: Final image
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai
COPY --from=backend /smart-daily /usr/local/bin/smart-daily
EXPOSE 9871
CMD ["smart-daily"]
