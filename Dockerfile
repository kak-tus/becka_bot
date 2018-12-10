FROM golang:1.11.2-alpine3.8 AS build

WORKDIR /go/src/github.com/kak-tus/becka_bot

COPY vendor ./vendor
COPY main.go .

RUN go install

FROM alpine:3.8

COPY --from=build /go/bin/becka_bot /usr/local/bin/becka_bot
COPY etc /etc/

RUN \
  adduser -DH user \
  \
  && apk add --no-cache \
    ca-certificates

USER user

ENV \
  BECKA_REDIS_ADDR= \
  BECKA_TELEGRAM_TOKEN= \
  BECKA_TELEGRAM_URL=\
  BECKA_TELEGRAM_PATH= \
  BECKA_TELEGRAM_PROXY=

EXPOSE 8080

CMD ["/usr/local/bin/becka_bot"]
