/*
Created © 2016-04-22 20:13 by @radaiming
// listen local DNS query
listenRequest -> taskChan

// listen websocket response
listenResponse -> retChan

// main
select {
<- taskChan
    go sendRequest
<- retChan
    go sendResult
}
*/

package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

var debug bool

func byteToDomain(data []byte) string {
	var domain string
	length := data[0]
	for i := 1; data[i] != 0x00; i++ {
		if length == 0 {
			domain += "."
			length = data[i]
		} else {
			domain += string(data[i : i+1])
			length -= 1
		}
	}
	return domain
}

func pingForever(wsConn *websocket.Conn) {
	ticker := time.NewTicker(time.Second * 30)
	for _ = range ticker.C {
		err := wsConn.WriteControl(websocket.PingMessage, []byte{0x00}, time.Now().Add(time.Second*30))
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func listenRequest(udpConn *net.UDPConn, taskChan chan []byte) {
	for {
		data := make([]byte, 1500)
		rLength, clientAddr, err := udpConn.ReadFromUDP(data)
		if err != nil {
			log.Println(err)
			continue
		}
		if debug {
			log.Println("querying " + byteToDomain(data[12:]))
		}
		// https://golang.org/ref/spec#Passing_arguments_to_..._parameters
		taskChan <- append(append([]byte(clientAddr.String()), []byte{0x00, 0x00}...), data[:rLength]...)
	}
}

func listenResponse(wsConn *websocket.Conn, retChan chan []byte) {
	for {
		_, data, err := wsConn.ReadMessage()
		if err != nil {
			log.Panicln(err)
			continue
		}
		retChan <- data
	}
}

func sendRequest(wsConn *websocket.Conn, taskChan chan []byte) {
	for {
		data := <-taskChan
		err := wsConn.SetWriteDeadline(time.Now().Add(time.Second * 10))
		if err != nil {
			log.Println(err)
			continue
		}
		err = wsConn.WriteMessage(websocket.BinaryMessage, data)
		if err != nil {
			log.Panicln(err)
			continue
		}
	}
}

func sendResult(udpConn *net.UDPConn, data []byte) {
	index := bytes.Index(data, []byte{0x00, 0x00})
	if index < 0 {
		log.Println("index error for returned packet")
		return
	}
	clientAddr := data[:index]
	clientAddrPtr, err := net.ResolveUDPAddr("udp", string(clientAddr))
	if err != nil {
		log.Println(err)
		return
	}
	realData := data[index+2:]
	udpConn.WriteToUDP(realData, clientAddrPtr)
	if debug {
		domain := byteToDomain(realData[12:])
		log.Println(fmt.Sprintf("result of %s sent to %s", domain, string(clientAddr)))
	}
}

func main() {
	var serverAddr, bindAddr string
	var bindPort int
	flag.StringVar(&serverAddr, "c", "", "set server url, like wss://test.com/dns")
	flag.StringVar(&bindAddr, "b", "127.0.0.1", "bind to this address, default to 127.0.0.1")
	flag.IntVar(&bindPort, "p", 5353, "bind to this port, default to 5353")
	flag.BoolVar(&debug, "d", false, "enable debug outputing")
	flag.Parse()

	// when query comes in, we put into this channel
	taskChan := make(chan []byte, 512)
	// when result comes back, we put into this channel
	retChan := make(chan []byte, 512)

	udpAddrPtr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", bindAddr, bindPort))
	if err != nil {
		log.Panicln(err)
	}
	udpConn, err := net.ListenUDP("udp", udpAddrPtr)
	if err != nil {
		log.Panicln(err)
	} else {
		log.Println(fmt.Sprintf("listening on %s:%d", bindAddr, bindPort))
		defer udpConn.Close()
	}

	wsConn, _, err := websocket.DefaultDialer.Dial(serverAddr, nil)
	if err != nil {
		log.Panicln(err)
	} else {
		log.Println("connected to " + serverAddr)
		defer wsConn.Close()
	}

	go pingForever(wsConn)
	go listenRequest(udpConn, taskChan)
	go listenResponse(wsConn, retChan)
	go sendRequest(wsConn, taskChan)

	for {
		data := <-retChan
		go sendResult(udpConn, data)
	}
}
