package main

import (
	"bytes"
	"code.google.com/p/goprotobuf/proto"
	"encoding/binary"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/mrdooz/swarm2/protocol"
	"hash/fnv"
	"log"
	"net/http"
)

var (
	conn            websocket.Conn
	clientId        uint32
	nextToken       uint32
	requestHandlers map[uint32]chan []byte = make(map[uint32]chan []byte)
)

func checkOrigin(r *http.Request) bool {
	return true
}

func ConnectRequestHandler(c chan []byte) {
	for {
		buf := <-c
		request := swarm.ConnectionRequest{}
		if err := proto.Unmarshal(buf, &request); err != nil {
			fmt.Println(err)
		}
		fmt.Printf("got stuff :) %d\n", request.GetDummy())

	}
}

func handler(w http.ResponseWriter, r *http.Request) {

	// upgrade the request to websocket
	upgrader := &websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}

	upgrader.CheckOrigin = checkOrigin
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	for {
		// wait for packet
		_, buf, err := conn.ReadMessage()
		if err != nil {
			fmt.Print(err)
			return
		}

		log.Printf("recv %d bytes\n", len(buf))

		// create reader, and parse header size
		reader := bytes.NewReader(buf)
		var headerSize int16
		if err = binary.Read(reader, binary.BigEndian, &headerSize); err != nil {
			fmt.Print(err)
			return
		}

		// parse header
		header := swarm.Header{}
		if err = proto.Unmarshal(buf[2:2+headerSize], &header); err != nil {
			fmt.Println(err)
			return
		}

		if !header.GetIsResponse() {
			// look up a channel for the hash
			ch := requestHandlers[header.GetMethodHash()]
			ch <- buf[2+headerSize:]
		}

		fmt.Printf("header size: %d, hash: %x, token: %d\n",
			headerSize, header.GetMethodHash(), header.GetToken())

		// find the reader for the response

		// parse body
		/*
				body := swarm.ConnectionResponse{}
				if err = proto.Unmarshal(buf[2+headerSize:], &body); err != nil {
					fmt.Println(err)
					return
				}
			fmt.Printf("connection id: %d\n", body.GetConnectionId())
		*/

	}
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func main() {
	log.Println("** server started")
	c := make(chan []byte)
	requestHandlers[hash("swarm.ConnectionRequest")] = c
	go ConnectRequestHandler(c)

	h := fnv.New32a()
	h.Write([]byte("magnus"))
	log.Print(h.Sum32())
	//	log.Print(hash.New32a().)

	http.HandleFunc("/", handler)
	//	http.HandleFunc("/ws", serveWs)
	if err := http.ListenAndServe("localhost:8080", nil); err != nil {
		log.Fatal(err)
	}

}
