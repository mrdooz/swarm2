/*
	Hub
		- handle websocket upgrade requests
		- start ClientConnection for each connection

	ClientConnection
		- go routine for reading
			- unmarshal message, send processed data to run loop channel
		- go routine for writing

		- run loop selecting on service channel
*/

package main

import (
	"github.com/gorilla/websocket"
	"github.com/mrdooz/swarm2/protocol"
	"log"
	"net/http"
)

var (
	hub     Hub
	gameMgr GameManager
)

// todo: rename request/response
type ProtoRequest struct {
	header swarm.Header
	body   []byte
}

type ProtoMessage struct {
	methodHash uint32
	isResponse bool
	body       []byte
}

func checkOrigin(r *http.Request) bool {
	return true
}

// Handle websocket handshake, upgrade the connection, and start
// a connection goroutine
func websocketHandler(w http.ResponseWriter, r *http.Request) {

	upgrader := &websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}

	upgrader.CheckOrigin = checkOrigin
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("[WS] Client connected")

	hub.clientConnected <- conn
}

func main() {
	log.Println("server started")
	go gameMgr.run()
	go hub.run()

	http.HandleFunc("/", websocketHandler)
	if err := http.ListenAndServe("localhost:8080", nil); err != nil {
		log.Fatal(err)
	}

}
