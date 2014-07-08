package main

import (
	"code.google.com/p/goprotobuf/proto"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/mrdooz/swarm2/protocol"
	"log"
	"net/http"
)

var (
	conn websocket.Conn
)

func checkOrigin(r *http.Request) bool {
	return true
}

func handler(w http.ResponseWriter, r *http.Request) {

	upgrader := &websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}

	upgrader.CheckOrigin = checkOrigin
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	var conResp = connection.ConnectionResponse{}

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			return
		}

		proto.Unmarshal(p, &conResp)
		fmt.Println(conResp.GetConnectionId())

		if err = conn.WriteMessage(messageType, p); err != nil {
			return
		}
	}
}

func main() {
	log.Println("** server started")

	http.HandleFunc("/", handler)
	//	http.HandleFunc("/ws", serveWs)
	if err := http.ListenAndServe("localhost:8080", nil); err != nil {
		log.Fatal(err)
	}

}
