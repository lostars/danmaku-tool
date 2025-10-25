FROM alpine:3.22 AS goreleaser

# goreleaser build产物路径，会单独放一个临时文件夹，不是dist下
ARG TARGETPLATFORM
COPY "$TARGETPLATFORM/danmaku" /usr/local/bin/
# 本地构建
#COPY "bin/danmaku" /usr/local/bin

EXPOSE 8089
ENTRYPOINT ["/usr/local/bin/danmaku", "server", "-c", "/app/config.yaml"]