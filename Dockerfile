FROM 192.168.10.67/egame/alpine:3.16

WORKDIR /app

COPY --chmod=755 stress /app/stress
COPY --chmod=644 configs/config.yaml /app/configs/config.yaml

RUN mkdir -p /app/log && chmod 755 /app/log

EXPOSE 8001 9001

HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD pgrep stress || exit 1

CMD ["/app/stress", "-conf", "/app/configs"]