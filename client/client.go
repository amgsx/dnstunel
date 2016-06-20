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
	"strconv"
	"strings"
)

var debug bool

type Client struct {
	taskChan   chan []byte
	retChan    chan []byte
	quitChan   chan int
	bindAddr   string
	bindPort   int
	serverAddr string
	wsConn     *websocket.Conn
	listenConn *net.UDPConn
}

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

func (c *Client) pingForever() {
	ticker := time.NewTicker(time.Second * 30)
	for _ = range ticker.C {
		err := c.wsConn.WriteControl(websocket.PingMessage, []byte{0x00}, time.Now().Add(time.Second*5))
		if err != nil {
			log.Println("fail to ping websocket server:", err)
			c.quitChan <- 1
			return
		}
	}
}

func (c *Client) listenRequest() {
	for {
		data := make([]byte, 1500)
		rLength, clientAddr, err := c.listenConn.ReadFromUDP(data)
		if err != nil {
			log.Printf("goroutine for %s error reading client request: %s\n", c.serverAddr, err.Error())
			c.quitChan <- 1
			return
		}
		if debug {
			log.Println("querying " + byteToDomain(data[12:]))
		}
		// https://golang.org/ref/spec#Passing_arguments_to_..._parameters
		c.taskChan <- append(append([]byte(clientAddr.String()), []byte{0x00, 0x00}...), data[:rLength]...)
	}
}

func (c *Client) listenResponse() {
	for {
		_, data, err := c.wsConn.ReadMessage()
		if err != nil {
			log.Printf("goroutine for %s error reading from websocket: %s\n", c.serverAddr, err.Error())
			c.quitChan <- 1
			return
		}
		c.retChan <- data
	}
}

func (c *Client) sendRequest() {
	for {
		select {
		case <-c.quitChan:
			return
		case data := <-c.taskChan:
			err := c.wsConn.SetWriteDeadline(time.Now().Add(time.Second * 10))
			if err != nil {
				log.Printf("goroutine for %s error setting deadline for websocket writing: %s\n", c.serverAddr, err.Error())
			}
			err = c.wsConn.WriteMessage(websocket.BinaryMessage, data)
			if err != nil {
				log.Printf("goroutine for %s error writing message to websocket: %s\n", c.serverAddr, err.Error())
				close(c.quitChan)
				return
			}
		}
	}
}

func (c *Client) sendResult(data []byte) {
	index := bytes.Index(data, []byte{0x00, 0x00})
	if index < 0 {
		log.Printf("goroutine for %s index error for returned packet\n", c.serverAddr)
		return
	}
	clientAddr := data[:index]
	clientAddrPtr, err := net.ResolveUDPAddr("udp", string(clientAddr))
	if err != nil {
		log.Println(err)
		return
	}
	realData := data[index+2:]
	_, err = c.listenConn.WriteToUDP(realData, clientAddrPtr)
	if err != nil {
		log.Printf("goroutine for %s error sending result back to client: %s\n", c.serverAddr, err.Error())
	} else if debug {
		domain := byteToDomain(realData[12:])
		log.Println(fmt.Sprintf("result of %s from %s sent to %s", domain, c.serverAddr, string(clientAddr)))
	}
}

func startOneClient(c Client) {
	// restart myself
	defer func() {
		time.Sleep(time.Second * 5)
		newClient := Client{}
		newClient.serverAddr = c.serverAddr
		newClient.taskChan = make(chan []byte, 512)
		newClient.retChan = make(chan []byte, 512)
		newClient.quitChan = make(chan int)
		newClient.bindAddr = c.bindAddr
		newClient.bindPort = c.bindPort
		go startOneClient(newClient)
	}()

	udpAddrPtr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", c.bindAddr, c.bindPort))
	if err != nil {
		log.Panicln("wrong binding address or port:", c.bindAddr, c.bindPort)
		return
	}
	c.listenConn, err = net.ListenUDP("udp", udpAddrPtr)
	if err != nil {
		log.Println(err)
		return
	} else {
		log.Println(fmt.Sprintf("listening on %s:%d", c.bindAddr, c.bindPort))
	}
	defer c.listenConn.Close()

	c.wsConn, _, err = websocket.DefaultDialer.Dial(c.serverAddr, nil)
	if err != nil {
		log.Println(err)
		return
	} else {
		log.Println("connected to " + c.serverAddr)
	}
	defer c.wsConn.Close()

	go c.pingForever()
	go c.listenRequest()
	go c.listenResponse()
	go c.sendRequest()
	for {
		select {
		case <-c.quitChan:
			return
		case data := <-c.retChan:
			go c.sendResult(data)
		}
	}
}

func main() {
	var serverAddrs, bindAddr, bindPorts string
	flag.StringVar(&serverAddrs, "c", "", "set server url, like wss://test1.com/dns,wss://test2.com/dns")
	flag.StringVar(&bindAddr, "b", "127.0.0.1", "bind to this address")
	flag.StringVar(&bindPorts, "p", "0", "bind to this port, like 5353,5354")
	flag.BoolVar(&debug, "d", false, "enable debug outputing")
	flag.Parse()

	if len(serverAddrs) == 0 || len(bindAddr) == 0 {
		log.Panicln("server address and port required")
	}
	serverSlice := strings.Split(serverAddrs, ",")
	portSlice := strings.Split(bindPorts, ",")
	if len(serverSlice) != len(portSlice) {
		log.Panicln("number of servers and binding ports not matched")
	}

	for i := range serverSlice {
		var err error
		c := Client{}
		c.bindAddr = bindAddr
		c.bindPort, err = strconv.Atoi(portSlice[i])
		if err != nil {
			log.Panicln("wrong port argument:", portSlice[i])
		}
		c.serverAddr = serverSlice[i]
		c.taskChan = make(chan []byte, 512)
		c.retChan = make(chan []byte, 512)
		c.quitChan = make(chan int)
		go startOneClient(c)
	}

	for {
		time.Sleep(time.Second)
	}
}
