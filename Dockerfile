FROM alpine:3.22 AS builder

ARG OUTPUT=dist
ARG BINARY=danmaku
ARG TARGETARCH
ARG TARGETOS
RUN apk add --no-cache tzdata
COPY ${OUTPUT}/${TARGETOS}/${TARGETARCH}/${BINARY} /usr/local/bin/danmaku

EXPOSE 8089
ENV TZ=Asia/Shanghai
ENTRYPOINT ["/usr/local/bin/danmaku", "server", "-c", "/app/config.yaml"]