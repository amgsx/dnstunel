FROM golang:1.6-alpine
MAINTAINER Ming Dai <radaiming@gmail.com>

ADD . /go/src/github.com/radaiming/DNS_Tunnel/

RUN apk update && apk add git
RUN cd /go/src/github.com/radaiming/DNS_Tunnel/ && go get ./...
RUN go install github.com/radaiming/DNS_Tunnel/server/

ENTRYPOINT ["/go/bin/server", "-b", "0.0.0.0"]

EXPOSE 5353/tcp
