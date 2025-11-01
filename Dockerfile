FROM alpine:3.22 AS goreleaser

RUN apk add --no-cache tzdata
COPY "dist/danmaku" /usr/local/bin

EXPOSE 8089
ENV TZ=Asia/Shanghai
ENTRYPOINT ["/usr/local/bin/danmaku", "server", "-c", "/app/config.yaml"]