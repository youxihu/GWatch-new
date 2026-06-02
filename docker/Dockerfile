FROM alpine:3.22.4

RUN apk add --no-cache ca-certificates tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone \
    && mkdir -p /app/acc/logs

ENV TZ=Asia/Shanghai

WORKDIR /app/acc

COPY ../bin/Gwatch ./GWatch

ENTRYPOINT ["./GWatch"]
CMD ["-c", "config.yml"]