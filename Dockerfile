# syntax=docker/dockerfile:1

##############################
# 阶段一：构建前端静态资源 #
##############################
FROM node:20-bullseye AS assets
WORKDIR /app

COPY package.json package-lock.json tailwind.config.js vite.config.js ./
RUN npm ci

COPY web ./web
RUN npm run build

#########################
# 阶段二：编译 Go 二进制 #
#########################
FROM golang:1.24-bullseye AS builder
WORKDIR /src

ENV CGO_ENABLED=1
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    pkg-config \
    sqlite3 \
    libsqlite3-dev \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
COPY --from=assets /app/web/static/dist ./web/static/dist

RUN go build -o /out/commitlog ./cmd/server

#########################
# 阶段三：运行镜像     #
#########################
FROM debian:bookworm-slim AS runner
WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    sqlite3 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/commitlog /usr/local/bin/commitlog
COPY --from=builder /src/web ./web

ENV PORT=8080 \
    DATABASE_PATH=/data/commitlog.db \
    GIN_MODE=release

EXPOSE 8080

CMD ["commitlog"]
