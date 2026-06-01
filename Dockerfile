# GWatch Docker 镜像
# 构建: docker build -t gwatch:4.0.1 .
# 运行: 见下方说明

FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai

COPY bin/Gwatch /usr/local/bin/gwatch

ENTRYPOINT ["/usr/local/bin/gwatch"]
CMD ["-c", "/etc/gwatch/config.yml"]