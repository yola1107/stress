FROM 192.168.10.67/egame/alpine:3.16

RUN apk add --no-cache tzdata || true && \
    if [ -f /usr/share/zoneinfo/Asia/Shanghai ]; then \
        cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
        echo "Asia/Shanghai" > /etc/timezone; \
    fi

WORKDIR /app

COPY server /app/server
RUN chmod +x /app/server

EXPOSE 8001
EXPOSE 9001

VOLUME /app/configs

CMD ["/app/server", "-conf", "/app/configs"]