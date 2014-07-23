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
	"./server_utils"
	"bytes"
	"code.google.com/p/goprotobuf/proto"
	"encoding/binary"
	"github.com/gorilla/websocket"
	"github.com/mrdooz/swarm2/protocol"
	"log"
	"net/http"
	"time"
)

var (
	CONNECTION_REQUEST_METHOD_HASH  uint32            = server_utils.Hash("swarm.ConnectionRequest")
	CONNECTION_RESPONSE_METHOD_HASH uint32            = server_utils.Hash("swarm.ConnectionResponse")
	PING_REQUEST_METHOD_HASH        uint32            = server_utils.Hash("swarm.PingRequest")
	PING_RESPONSE_METHOD_HASH       uint32            = server_utils.Hash("swarm.PingResponse")
	PLAYER_STATE_METHOD_HASH        uint32            = server_utils.Hash("swarm.PlayerState")
	methodHashToName                map[uint32]string = make(map[uint32]string)
)

var (
	connectionMgr ConnectionManager
	gameMgr       GameManager
)

type Vector2 struct {
	x float32
	y float32
}

type PlayerState struct {
	id  uint32
	acc Vector2
	vel Vector2
	pos Vector2
}

func (v *Vector2) fromProtocol(p *swarm.Vector2) {
	v.x = p.GetX()
	v.y = p.GetY()
}

func (v *Vector2) toProtocol(p *swarm.Vector2) {
	p = &swarm.Vector2{X: &v.x, Y: &v.y}
}

func (state *PlayerState) fromProtocol(p *swarm.PlayerState) {
	state.id = p.GetId()
	state.acc.fromProtocol(p.GetAcc())
	state.vel.fromProtocol(p.GetVel())
	state.pos.fromProtocol(p.GetPos())
}

func (state *PlayerState) toProtocol(p *swarm.PlayerState) {
	p = &swarm.PlayerState{Id: &state.id}
	state.acc.toProtocol(p.GetAcc())
	state.vel.toProtocol(p.GetVel())
	state.pos.toProtocol(p.GetPos())
}

type GameState struct {
	gameId  uint32
	players []PlayerState
}

func (state *GameState) toProtocol(p *swarm.GameState) {

	p = &swarm.GameState{GameId: proto.Uint32(state.gameId)}

	for _, player := range state.players {
		s := &swarm.PlayerState{}
		player.toProtocol(s)
		p.Players = append(p.Players, s)
	}
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

//----------------------
// GameService
type GameService struct {
	createGameRequest  chan CreateGameRequest
	createGameResponse chan CreateGameResponse

	gameState chan GameState
}

type CreateGameRequest struct {
	response   *chan CreateGameResponse
	createGame bool
}

type CreateGameResponse struct {
	newGame  bool
	playerId uint32
	gameId   uint32
	players  []uint32
}

//----------------------

type ClientConnection struct {
	clientId     uint32
	conn         *websocket.Conn
	outgoing     chan []byte
	disconnected chan bool
	gameService  GameService
}

type GameManager struct {
	nextGameId   uint32
	nextPlayerId uint32

	games   map[uint32]GameState
	players map[uint32]PlayerState

	playerState chan swarm.PlayerState

	gameService GameService
}

func (mgr *GameManager) run() {

	mgr.gameService = GameService{make(chan CreateGameRequest), nil, nil}
	mgr.playerState = make(chan swarm.PlayerState)

	select {
	case req := <-mgr.gameService.createGameRequest:
		*req.response <- CreateGameResponse{
			true,
			mgr.nextPlayerId,
			mgr.nextGameId,
			[]uint32{}}
		mgr.nextGameId++
		mgr.nextPlayerId++
		break

	case state := <-mgr.playerState:
		// look up the player/game
		player := mgr.players[state.GetId()]
		player.fromProtocol(&state)
		break

		// case <-time.After(100 * time.Millisecond):
		// 	for _, g := range mgr.games {

		// 	}
		// 	break
	}
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

func (client *ClientConnection) writer() {

	for {
		// read from the outgoing channel, and send it on the websocket
		buf := <-client.outgoing
		if err := client.conn.WriteMessage(websocket.BinaryMessage, buf); err != nil {
			break
		}
	}
}

func (client *ClientConnection) reader() {
	conn := client.conn
	outgoing := client.outgoing

	for {
		_, buf, err := conn.ReadMessage()
		if err != nil {
			client.disconnected <- true
			log.Println("[CC-R] Disconnected")
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

				response := swarm.ConnectionResponse{}
				sendProtoMessage(
					outgoing,
					&response,
					makeHash("swarm.ConnectionRespose"),
					header.GetToken(),
					true)

				gameMgr.gameService.createGameRequest <- CreateGameRequest{
					&client.gameService.createGameResponse,
					request.GetCreateGame()}
				break

			case PLAYER_STATE_METHOD_HASH:
				state := swarm.PlayerState{}
				if err := proto.Unmarshal(body, &state); err != nil {
					log.Println(err)
				} else {
				}
				break
			}
		}
	}

}

func (client *ClientConnection) run() {

	client.gameService = GameService{
		nil,
		make(chan CreateGameResponse),
		make(chan GameState)}
	client.outgoing = make(chan []byte)
	client.disconnected = make(chan bool)

	go client.writer()
	go client.reader()

	var token uint32 = 0
	outgoing := client.outgoing

	done := false
	for !done {
		select {
		case response := <-client.gameService.createGameResponse:
			e := swarm.EnterGame{
				GameId:   proto.Uint32(response.gameId),
				PlayerId: proto.Uint32(response.playerId)}

			sendProtoMessage(outgoing, &e, makeHash("swarm.EnterGame"), token, false)
			token++
			break

		case <-time.After(10 * time.Second):
			sendProtoMessage(outgoing, &swarm.PingRequest{}, makeHash("swarm.PingRequest"), token, false)
			token++
			break

		case <-client.disconnected:
			done = true
			break
		}

	}

	log.Println("[CC] Disconnected")
	connectionMgr.clientDisconnected <- client.clientId
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

	for {
		select {
		case conn := <-mgr.clientConnected:
			log.Println("[CM] Client connected")
			// client connected, update structs and create the goroutine to
			// serve it
			clientId := mgr.nextClientId
			client := &ClientConnection{clientId: clientId, conn: conn}

			mgr.clients[clientId] = client
			mgr.nextClientId++
			go client.run()
			break

		case clientId := <-mgr.clientDisconnected:
			log.Println("[CM] Client disconnected")
			delete(mgr.clients, clientId)
			break
		}
	}

	log.Println("[CM] Exit")
}

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

	log.Println("[WS] Client connected")

	connectionMgr.clientConnected <- conn
}

func makeHash(str string) uint32 {
	h := server_utils.Hash(str)
	_, n := methodHashToName[h]
	if !n {
		methodHashToName[h] = str
		log.Printf("hash: %s => %d\n", str, h)
	}
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
