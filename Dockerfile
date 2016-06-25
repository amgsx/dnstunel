FROM alpine:3.4
MAINTAINER Ming Dai <radaiming@gmail.com>

ADD . /go/src/github.com/radaiming/DNS_Tunnel/
ENV GOPATH /go/

RUN apk add --no-cache go git && cd /go/src/github.com/radaiming/DNS_Tunnel/ && go get ./... && go install github.com/radaiming/DNS_Tunnel/server/ && apk del git go

ENTRYPOINT ["/go/bin/server", "-b", "0.0.0.0"]

EXPOSE 5353/tcp
