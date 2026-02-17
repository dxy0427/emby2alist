# 编译阶段
FROM golang:1.21-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# 静态编译，减小体积
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o emby2alist cmd/app/main.go

# 运行阶段
FROM alpine:latest

WORKDIR /app
# 安装时区数据和证书
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai

COPY --from=builder /src/emby2alist .

EXPOSE 8091
VOLUME ["/app/config.yaml"]

CMD ["./emby2alist"]