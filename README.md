# A simple DNS tunnel based on WebSocket

I write this to fix DNS pollution in China:

~~~~~~~~
DNS query <--> client <--> WebSocket <--> server <--> DNS server
~~~~~~~~

## Deploy on your server

~~~~~~~~
go get -u github.com/gorilla/websocket
cd server
go build
./server -p 9999
~~~~~~~~

Now it's listening on localhost 9999, next to config nginx, add a location for it:

~~~~~~~~
location /shining_tunnel {
    proxy_pass http://127.0.0.1:9999;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}
~~~~~~~~
Please use HTTPS, not only for security, but also to avoid possible breakage caused by ISP's performance tuning on HTTP.

## Deploy on Heroku
Clone this branch, then push:

~~~~~~~~
git push heroku go:master
~~~~~~~~

## Deplay on OpenShift
OpenShift does not support outgoing raw socket, so this program won't work, don't waste your time.

## Run client
Very simple:

~~~~~~~~
cd client
go build
./client -c wss://your-app-name.herokuapp.com -p 12345
~~~~~~~~

~~~~~~~~
$ dig +short twitter.com @127.0.0.1 -p 12345
199.59.150.7
199.59.149.230
199.59.150.39
199.59.149.198
~~~~~~~~

## Running inside docker containers

### Building a docker image

~~~~~~~~
$ docker build -t dnstunnel:latest .
~~~~~~~~

### Running as a server

~~~~~~~~
$ docker run -d --name=dserver -P dnstunnel:latest
~~~~~~~~

Please follow instructions above to set up nginx as an SSL termination.
To make fixed port mappings either on server or client, replace **-P/--publish-all** with **-p/--publish** [option](https://docs.docker.com/engine/reference/run/#expose-incoming-ports).
Otherwise you need to run `docker port` to figure out which port is actually listen on the host.
