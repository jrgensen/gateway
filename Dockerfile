FROM golang:1.13 AS builder

RUN mkdir -p /app/build
RUN mkdir -p /app/src
WORKDIR /app/src

COPY src .
RUN go get -v -d
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w' -o ../build/gateway

FROM alpine:3.8

RUN apk add -U --no-cache ca-certificates

RUN mkdir /app
WORKDIR /app

COPY --from=builder /app/build/gateway .

EXPOSE 80 443
CMD /app/gateway
#-port 80 -host local.pnorental.com -hostip `route | grep default | tr -s ' ' | cut -d' ' -f2`
