/*
Created © 2016-04-22 20:13 by @radaiming
*/

package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/gorilla/websocket"
)

func listenRequest(bindAddr string, bindPort int) {
	ln, err := net.Listen("udp", fmt.Sprintf("%s:%d", bindAddr, bindPort))
	if err != nil {
		panic(err)
	}
	for {
		conn, err := ln.Accept()
	}
}

func main() {
	var serverAddr = *flag.String("Server Address", "", "set server url, like wss://test.com/dns")
	var bindAddr = *flag.String("Bind Address", "127.0.0.1", "bind to this address, default to 127.0.0.1")
	var bindPort = *flag.Int("Bind Port", 5353, "bind to this port, default to 5353")
	flag.Parse()

	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt)

	conn, _, err := websocket.DefaultDialer.Dial(serverAddr, nil)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	go listenRequest(bindAddr, bindPort)
}
