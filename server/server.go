package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net"
	"net/http"
)

type Server struct {
	taskChan     chan []byte
	retChan      chan []byte
	debug        bool
	bindAddr     string
	bindPort     int
	conn         *websocket.Conn
	upstream     *net.UDPAddr
	upstreamAddr string
}

func byteToDomain(data []byte) string {
	var domain string
	index := bytes.Index(data, []byte{0x00, 0x00})
	if index < 0 {
		return "unknown"
	}
	realData := data[index+2+12:]
	length := realData[0]
	for i := 1; realData[i] != 0x00; i++ {
		if length == 0 {
			domain += "."
			length = realData[i]
		} else {
			domain += string(realData[i : i+1])
			length -= 1
		}
	}
	return domain
}

func (srv *Server) write() {
	for {
		data := <-srv.retChan
		err := srv.conn.WriteMessage(websocket.BinaryMessage, data)
		if err != nil {
			log.Println("error sending result: ", err)
		}
	}
}

func (srv *Server) query() {
	for {
		data := <-srv.taskChan
		tmp := make([]byte, len(data))
		copy(tmp, data)
		go func(data []byte) {
			index := bytes.Index(data, []byte{0x00, 0x00})
			if index < 0 {
				log.Println("indexing error for incomping query packet")
				return
			}
			queryData := data[index+2:]
			udpConn, err := net.DialUDP("udp4", nil, srv.upstream)
			_, err = udpConn.Write(queryData)
			if err != nil {
				log.Println("failed to send DNS query to upstream DNS server", err)
				return
			}
			defer udpConn.Close()
			// 2048 is enough?
			retData := make([]byte, 2048)
			n, err := udpConn.Read(retData)
			if err != nil {
				log.Println("failed to read DNS response from upstream DNS server", err)
				return
			}
			// the data to send back to client
			respData := make([]byte, 0, 2048+2+index)
			srv.retChan <- append(append(append(respData, data[:index]...), []byte{0x00, 0x00}...), retData[:n]...)
		}(tmp)
	}
}

func (srv *Server) listen(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{}
	var err error
	srv.conn, err = upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatalln("error upgrading to websocket protocol:", err)
	}
	fmt.Println("client connected from:", srv.conn.RemoteAddr())
	defer srv.conn.Close()
	for {
		messageType, data, err := srv.conn.ReadMessage()
		if err != nil {
			log.Println("error reading data from websocket:", err)
			// close connection
			break
		}
		if messageType == websocket.BinaryMessage {
			// ignore other control msg?
			if srv.debug {
				log.Println("querying", byteToDomain(data))
			}
			srv.taskChan <- data
		}
	}
}

func main() {
	srv := Server{}
	flag.StringVar(&srv.bindAddr, "b", "127.0.0.1", "bind to this address")
	flag.StringVar(&srv.upstreamAddr, "s", "8.8.8.8:53", "set upstream DNS server address and port")
	flag.IntVar(&srv.bindPort, "p", 5353, "bind to this port")
	flag.BoolVar(&srv.debug, "d", false, "enable debug outputing")
	flag.Parse()

	var err error
	srv.upstream, err = net.ResolveUDPAddr("udp4", srv.upstreamAddr)
	if err != nil {
		log.Panicln("error parsing upstream DNS server addr:", err)
	}
	srv.taskChan = make(chan []byte, 512)
	srv.retChan = make(chan []byte, 512)

	go srv.write()
	go srv.query()
	http.HandleFunc("/", srv.listen)
	fmt.Printf("listening request on %s:%d\n", srv.bindAddr, srv.bindPort)
	fmt.Println("upstream DNS server:", srv.upstreamAddr)
	log.Fatalln(http.ListenAndServe(fmt.Sprintf("%s:%d", srv.bindAddr, srv.bindPort), nil))
}
