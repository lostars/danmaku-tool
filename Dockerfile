FROM alpine:3.22 AS builder

ARG OUTPUT=dist
ARG BINARY=danmaku
ARG TARGETARCH
ARG TARGETOS
RUN apk add --no-cache tzdata
COPY ${OUTPUT}/${TARGETOS}/${TARGETARCH}/${BINARY} /usr/local/bin/danmaku

ARG PUID=1001
ARG PGID=1001
ARG USER=${BINARY}
ARG GROUP=${BINARY}
RUN addgroup -g ${PGID} ${GROUP} && \
    adduser -D -u ${PUID} -G ${GROUP} ${USER}
USER ${USER}

EXPOSE 8089
ENV TZ=Asia/Shanghai
ENTRYPOINT ["/usr/local/bin/danmaku", "server", "-c", "/app/config.yaml"]