/*
Created © 2016-04-22 20:13 by @radaiming
*/

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/gorilla/websocket"
)

func listenRequest(udpConn *net.UDPConn, taskChan chan []byte) {
	for {
		data := make([]byte, 1500)
		rLength, clientAddr, err := udpConn.ReadFromUDP(data)
		if err != nil {
			log.Println(err)
			continue
		}
		taskChan <- append(append([]byte(clientAddr.String()), []byte{0x00, 0x00}...), data[:rLength]...)
	}
}

func main() {
	var serverAddr = *flag.String("Server Address", "", "set server url, like wss://test.com/dns")
	var bindAddr = *flag.String("Bind Address", "127.0.0.1", "bind to this address, default to 127.0.0.1")
	var bindPort = *flag.Int("Bind Port", 5353, "bind to this port, default to 5353")
	flag.Parse()

	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt)
	// when query comes in, we put into this channel
	taskChan := make(chan []byte, 512)

	udpAddrPtr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", bindAddr, bindPort))
	if err != nil {
		panic(err)
	}
	udpConn, err := net.ListenUDP("udp", udpAddrPtr)
	if err != nil {
		panic(err)
	}
	defer udpConn.Close()

	wsConn, _, err := websocket.DefaultDialer.Dial(serverAddr, nil)
	if err != nil {
		panic(err)
	}
	defer wsConn.Close()

	go listenRequest(udpConn, taskChan)
}
