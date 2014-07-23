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
	CONNECTION_REQUEST_METHOD_HASH  uint32 = Hash("swarm.ConnectionRequest")
	CONNECTION_RESPONSE_METHOD_HASH uint32 = Hash("swarm.ConnectionResponse")
	PING_REQUEST_METHOD_HASH        uint32 = Hash("swarm.PingRequest")
	PING_RESPONSE_METHOD_HASH       uint32 = Hash("swarm.PingResponse")
	PLAYER_STATE_METHOD_HASH        uint32 = Hash("swarm.PlayerState")

	methodHashToName map[uint32]string = make(map[uint32]string)
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

//----------------------
// GameService
type GameService struct {
	createGameRequest  chan CreateGameRequest
	createGameResponse chan CreateGameResponse

	gameState chan GameState
}

type CreateGameRequest struct {
	response   *chan CreateGameResponse
	gameState  *chan GameState
	createGame bool
}

type CreateGameResponse struct {
	newGame  bool
	playerId uint32
	gameId   uint32
	players  []uint32
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
