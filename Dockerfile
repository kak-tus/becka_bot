FROM golang:1.13.2-alpine3.10 AS build

WORKDIR /go//becka_bot

COPY go.mod .
COPY go.sum .
COPY main.go .

RUN go build -o /go/bin/becka_bot

FROM alpine:3.10

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
