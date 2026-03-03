FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai
COPY bin/smart-daily /usr/local/bin/smart-daily
EXPOSE 9871
CMD ["smart-daily"]
