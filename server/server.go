/*
	hub start
		- handle websocket upgrade requests
		- start ClientConnection for each connection

		ClientConnection
			- go routine for reading
				- unmarshal message, send processed data to run loop channel
			- go routine for writing

			- run loop selecting on channel
*/

package main

import (
	"./server_utils"
	"bytes"
	"code.google.com/p/goprotobuf/proto"
	"encoding/binary"
	"github.com/gorilla/websocket"
	"github.com/mrdooz/swarm2/protocol"
	"log"
	"net/http"
	//	"time"
)

type PlayerId uint32
type GameId uint32

type Vector2 struct {
	x float32
	y float32
}

type PlayerState struct {
	pos Vector2
	vel Vector2
	acc Vector2
}

type PlayerInfo struct {
	id    PlayerId
	state PlayerState
}

type GameState struct {
	id      GameId
	players []PlayerInfo
}

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

type GameService struct {
	createGameResponse chan GameId
}

type ClientConnection struct {
	clientId    uint32
	conn        *websocket.Conn
	outgoing    chan []byte
	gameService GameService
}

type GameRequest struct {
	response   *chan GameId
	createGame bool
}

type GameManager struct {
	nextGameId   GameId
	nextPlayerId PlayerId

	games   map[GameId]GameState
	players map[PlayerId]PlayerInfo

	createGameRequest chan GameRequest
}

func (mgr *GameManager) run() {

	mgr.createGameRequest = make(chan GameRequest)

	select {
	case req := <-mgr.createGameRequest:
		*req.response <- mgr.nextGameId
		mgr.nextGameId++
		break
	}
}

var (
	CONNECTION_REQUEST_METHOD_HASH  uint32 = server_utils.Hash("swarm.ConnectionRequest")
	CONNECTION_RESPONSE_METHOD_HASH uint32 = server_utils.Hash("swarm.ConnectionResponse")
	PING_REQUEST_METHOD_HASH        uint32 = server_utils.Hash("swarm.PingRequest")
	PING_RESPONSE_METHOD_HASH       uint32 = server_utils.Hash("swarm.PingResponse")
)

func createProtoHeader(
	methodHash uint32,
	token uint32,
	isResponse bool) (err error, headerSize uint16, headerBuf []byte) {

	headerSize = 0
	header := &swarm.Header{
		MethodHash: proto.Uint32(methodHash),
		Token:      proto.Uint32(token),
		IsResponse: proto.Bool(isResponse),
	}

	headerBuf, err = proto.Marshal(header)
	if err == nil {
		headerSize = uint16(len(headerBuf))
	}
	return
}

func sendProtoMessage(
	outgoing chan []byte,
	message ProtobufMessage,
	methodHash uint32,
	token uint32,
	isResponse bool) bool {

	pp := ProtoMessage{methodHash: methodHash, isResponse: isResponse}
	var err error
	pp.body, err = proto.Marshal(message)
	if err != nil {
		log.Println(err)
		return false
	}

	header := &swarm.Header{
		MethodHash: proto.Uint32(methodHash),
		Token:      proto.Uint32(token),
		IsResponse: proto.Bool(isResponse),
	}

	headerBuf, err := proto.Marshal(header)
	if err != nil {
		log.Println(err)
		return false
	}

	headerSize := uint16(len(headerBuf))

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, &headerSize)
	binary.Write(buf, binary.BigEndian, headerBuf)
	binary.Write(buf, binary.BigEndian, pp.body)

	outgoing <- buf.Bytes()

	return true
}

func (client *ClientConnection) run() {

	client.gameService = GameService{make(chan GameId)}
	client.outgoing = make(chan []byte)

	// make channel and goroutine to send data over the websocket
	go func(outgoing chan []byte) {
		for {
			buf := <-outgoing
			client.conn.WriteMessage(websocket.BinaryMessage, buf)
		}
	}(client.outgoing)

	// reader goroutine
	go func(client *ClientConnection) {

		conn := client.conn

		for {
			_, buf, err := conn.ReadMessage()
			if err != nil {
				connectionMgr.clientDisconnected <- client.clientId
				log.Println("disconnected")
				break
			}

			err, headerSize, header := server_utils.ParseProtoHeader(buf)
			if err != nil {
				log.Println("error parsing proto header")
			}
			log.Printf("recv: %d, %x", headerSize, header.GetMethodHash())

			body := buf[2+headerSize:]
			if header.GetIsResponse() {

				switch header.GetMethodHash() {
				case PING_RESPONSE_METHOD_HASH:
					break
				}

			} else {

				switch header.GetMethodHash() {
				case CONNECTION_REQUEST_METHOD_HASH:
					request := swarm.ConnectionRequest{}
					if err := proto.Unmarshal(body, &request); err != nil {
						log.Println(err)
						return
					}

					gameMgr.createGameRequest <- GameRequest{
						&client.gameService.createGameResponse,
						request.GetCreateGame()}
					break

				case PING_REQUEST_METHOD_HASH:
					break

				}
			}
		}

	}(client)

	var token uint32 = 0
	outgoing := client.outgoing

	for {
		select {
		case gameId := <-client.gameService.createGameResponse:
			log.Printf("create game response: %d", gameId)
			e := swarm.EnterGame{
				Id: &swarm.PlayerId{
					GameId: proto.Uint32(uint32(gameId)), PlayerId: proto.Uint32(0)}}

			sendProtoMessage(outgoing, &e, makeHash("swarm.EnterGame"), token, false)
			token++

			break
		}
	}

}

type ConnectionManager struct {
	clientConnected    chan *websocket.Conn
	clientDisconnected chan uint32

	nextClientId uint32
	clients      map[uint32]*ClientConnection
}

func (mgr *ConnectionManager) run() {

	mgr.clientConnected = make(chan *websocket.Conn)
	mgr.clientDisconnected = make(chan uint32)
	mgr.clients = make(map[uint32]*ClientConnection)

	select {
	case conn := <-mgr.clientConnected:
		// client connected, update structs and create the goroutine to
		// serve it
		clientId := mgr.nextClientId
		client := &ClientConnection{clientId: clientId, conn: conn}

		mgr.clients[clientId] = client
		mgr.nextClientId++
		go client.run()

		break

	case clientId := <-mgr.clientDisconnected:
		log.Println(clientId)
		break
	}

}

var (
	methodHashToName map[uint32]string = make(map[uint32]string)

	// todo: split into per connection and per game
	connectedClients map[uint32]ClientConnection = make(map[uint32]ClientConnection)

	clientConnected    chan ClientConnection = make(chan ClientConnection)
	clientDisconnected chan ClientConnection = make(chan ClientConnection)

	nextGameId   GameId
	nextPlayerId PlayerId
	games        map[GameId]GameState    = make(map[GameId]GameState)
	players      map[PlayerId]PlayerInfo = make(map[PlayerId]PlayerInfo)

	connectionMgr ConnectionManager
	gameMgr       GameManager
)

func checkOrigin(r *http.Request) bool {
	return true
}

type ProtobufMessage interface {
	ProtoMessage()
	Reset()
	String() string
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

	connectionMgr.clientConnected <- conn
}

func makeHash(str string) uint32 {
	h := server_utils.Hash(str)
	methodHashToName[h] = str
	return h
}

func main() {
	log.Println("server started")
	go gameMgr.run()
	go connectionMgr.run()

	http.HandleFunc("/", websocketHandler)
	if err := http.ListenAndServe("localhost:8080", nil); err != nil {
		log.Fatal(err)
	}

}
