FROM docker.io/library/golang:1.24.5-alpine3.21
#FROM --platform=linux/amd64 alpine:3.16
#FROM docker.io/library/debian:11-slim
#FROM e5e3fc0e0386
#FROM alpine:3.16
#FROM 192.168.10.67/library/alpine


#FROM localhost/debian:11-slim

ENV GOPROXY=https://goproxy.cn,direct \
    GOROOT=/usr/local/go

#    PATH=/usr/local/go/bin:$PATH \
#    CGO_ENABLED=0

WORKDIR /workspace

# 1️⃣ 安装 Podman 和基础工具（仅 apt，走你现有源）
RUN apk update && \
    apk add --no-cache \
        podman \
        docker-cli \
        ca-certificates \
        git \
        bash \
        shadow-uidmap \
        fuse \
        procps \
        iproute2 && \
    rm -rf /var/lib/apt/lists/*

# 2️⃣ 拷贝离线 Go 包
#COPY go  /usr/local/go

# 3️⃣ 安装 Go（完全离线）
#RUN tar -C /usr/local -xzf /tmp/go1.24.5.linux-amd64.tar.gz && \
#    rm -f /tmp/go1.24.5.linux-amd64.tar.gz

# 4️⃣ 校验
#RUN go version && podman --version

CMD ["bash"]
