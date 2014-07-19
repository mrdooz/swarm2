package main

import (
	"./server_utils"
	//	"bytes"
	"code.google.com/p/goprotobuf/proto"
	//	"encoding/binary"
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

type ClientConnection struct {
	clientId  uint32
	conn      *websocket.Conn
	nextToken uint32

	//	requestChannels  map[uint32]chan ProtoRequest = make(map[uint32]chan ProtoRequest)
	//	outgoingChannel  chan ProtoMessage            = make(chan ProtoMessage)
}

/*
func Handler(header swarm.Header, body []byte) {

	// unmarshal the body
	request := swarm.ConnectionRequest{}
	if err := proto.Unmarshal(body, &request); err != nil {
		log.Println(err)
		return
	}
	log.Println("Connection request")

	// check if the player wants to create a new game, or join an existing
	if request.GetCreateGame() {

		gameMgr.createGameRequest <- true
		gameId := <-gameMgr.creatGameResponse

		// player := PlayerInfo{id: nextPlayerId}
		// game := GameState{id: nextGameId}

		// game.players = append(game.players, player)
		// players[nextPlayerId] = player

		// sendEnterGame(nextPlayerId, nextGameId, game.players)

		// nextPlayerId++
		// nextGameId++

	} else {

	}

	// send the response

	for {
		p := <-c
		request := swarm.ConnectionRequest{}
		if err := proto.Unmarshal(p.body, &request); err != nil {
			log.Println(err)
		}
		log.Println("Connection request")

		response := &swarm.ConnectionResponse{}
		sendProtoMessage(response, p.header.GetMethodHash(), true)

	}
}
*/
type GameManager struct {
	nextGameId   GameId
	nextPlayerId PlayerId

	games   map[GameId]GameState
	players map[PlayerId]PlayerInfo

	createGameRequest chan bool
	creatGameResponse chan GameId
}

func (mgr *GameManager) run() {

	mgr.createGameRequest = make(chan bool)
	mgr.creatGameResponse = make(chan GameId)

	select {
	case <-mgr.createGameRequest:
		mgr.creatGameResponse <- mgr.nextGameId
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

func (client *ClientConnection) run() {

	// make channel and goroutine to send data over the websocket
	outgoing := make(chan []byte)
	go func() {
		for {
			buf := <-outgoing
			client.conn.WriteMessage(websocket.BinaryMessage, buf)
		}
	}()

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

				if request.GetCreateGame() {

				} else {
					// join existing game
				}

				break

			case PING_REQUEST_METHOD_HASH:
				break

			}

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
		client := &ClientConnection{clientId, conn, 0}

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

/*
func sendProtoMessage(message ProtobufMessage, methodHash uint32, isResponse bool) {

	pp := ProtoMessage{methodHash: methodHash, isResponse: isResponse}
	var err error
	pp.body, err = proto.Marshal(message)
	if err == nil {
		log.Println("sendProtoMessage")
		outgoingChannel <- pp
	} else {
		log.Println(err)
	}
}

func sendEnterGame(playerId PlayerId, gameId GameId, players []PlayerInfo) {
	e := swarm.EnterGame{
		Id: &swarm.PlayerId{
			GameId: proto.Uint32(uint32(gameId)), PlayerId: proto.Uint32(uint32(playerId))}}
	sendProtoMessage(&e, makeHash("swarm.EnterGame"), false)
}

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

func connectionProc(conn *websocket.Conn) {

	// create channel for incoming messages, and spawn goroutine to process them
	disconnected := make(chan error)
	incoming := make(chan []byte)
	go func(ch chan []byte) {
		// wait for packet
		_, buf, err := conn.ReadMessage()
		if err != nil {
			disconnected <- err
			log.Println("disconnected")
			return
		}
		ch <- buf
	}(incoming)

	// create the ping response channel
	pingResponseChannel := make(chan ProtoRequest)
	requestChannels[makeHash("swarm.PingResponse")] = pingResponseChannel

	for {

		// handle incoming and outgoing packets
		select {
		case err := <-disconnected:
			log.Print(err)
			break

		// send a ping every second to check for timeouts
		case <-time.After(1 * time.Second):
			log.Println("ping")
			p := swarm.PingRequest{}
			go sendProtoMessage(&p, makeHash("swarm.PingRequest"), false)
			break

		case <-pingResponseChannel:
			log.Println("pong")
			break

		case buf := <-incoming:
			log.Printf("recv %d bytes\n", len(buf))

			// create reader, and parse header size
			err, headerSize, header := parseProtoHeader(buf)

			if err != nil {
				log.Println(err)

			} else {
				log.Printf("header size: %d, hash: %x, token: %d\n",
					headerSize, header.GetMethodHash(), header.GetToken())

				if !header.GetIsResponse() {
					// look up a channel for the hash
					ch := requestChannels[header.GetMethodHash()]
					ch <- ProtoRequest{header: header, body: buf[2+headerSize:]}
				} else {
					// packet is a request
				}
			}

		case req := <-outgoingChannel:
			log.Println("sending outgoing")

			err, headerSize, headerBuf := createProtoHeader(req.methodHash, nextToken, req.isResponse)
			if err != nil {
				log.Println("Error marshaling proto header")
				continue
			}
			nextToken++

			buf := new(bytes.Buffer)
			binary.Write(buf, binary.BigEndian, &headerSize)
			binary.Write(buf, binary.BigEndian, headerBuf)
			binary.Write(buf, binary.BigEndian, req.body)

			if err := conn.WriteMessage(websocket.BinaryMessage, buf.Bytes()); err != nil {
				log.Println("disconnect")
				log.Println(err)
			}
		}
	}

}
*/
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
	/*
		// assign client id, and start goroutine
		connectedClients[nextClientId] = ClientConnection{nextClientId, conn}
		nextClientId++

		// todo: update client connection struct etc
		go connectionProc(conn)*/
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
