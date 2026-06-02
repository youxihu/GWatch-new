FROM alpine:3.22.4

RUN apk add --no-cache ca-certificates tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone \
    && mkdir -p /app/acc/logs

ENV TZ=Asia/Shanghai
WORKDIR /app/acc

# buildx 自动注入
ARG TARGETARCH

# 根据平台自动选择二进制
COPY bin/linux_${TARGETARCH}/Gwatch ./GWatch

ENTRYPOINT ["./GWatch"]
CMD ["-c", "config.yml"]