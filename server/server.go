package main

import (
	"bytes"
	"code.google.com/p/goprotobuf/proto"
	"encoding/binary"
	"github.com/gorilla/websocket"
	"github.com/mrdooz/swarm2/protocol"
	"hash/fnv"
	"log"
	"net/http"
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

type ProtoResponse struct {
	methodHash uint32
	isResponse bool
	body       []byte
}

type ClientConnection struct {
	clientId uint32
	conn     *websocket.Conn
}

var (
	nextToken uint32

	// todo: split into per connection and per game
	requestChannels  map[uint32]chan ProtoRequest = make(map[uint32]chan ProtoRequest)
	responseChannel  chan ProtoResponse           = make(chan ProtoResponse)
	connectedClients map[uint32]ClientConnection  = make(map[uint32]ClientConnection)

	nextClientId uint32
	nextGameId   GameId
	nextPlayerId PlayerId
	games        map[GameId]GameState    = make(map[GameId]GameState)
	players      map[PlayerId]PlayerInfo = make(map[PlayerId]PlayerInfo)
)

type ConnectionManager struct {
}

type GameManager struct {
	nextGameId   GameId
	nextPlayerId PlayerId

	games   map[GameId]GameState
	players map[PlayerId]PlayerInfo
}

func checkOrigin(r *http.Request) bool {
	return true
}

type Marshaler interface {
	ProtoMessage()
	Reset()
	String() string
}

func sendProtoResponse(response Marshaler, methodHash uint32) {

	pp := ProtoResponse{methodHash: methodHash, isResponse: true}
	var err error
	pp.body, err = proto.Marshal(response)
	if err == nil {
		responseChannel <- pp
	} else {
		log.Println(err)
	}
}

func sendOutgoing(response Marshaler, methodHash uint32) {

	pp := ProtoResponse{methodHash: methodHash, isResponse: false}
	var err error
	pp.body, err = proto.Marshal(response)
	if err == nil {
		responseChannel <- pp
	} else {
		log.Println(err)
	}
}

func sendEnterGame(playerId PlayerId, gameId GameId, players []PlayerInfo) {
	e := swarm.EnterGame{
		Id: &swarm.PlayerId{
			GameId: proto.Uint32(uint32(gameId)), PlayerId: proto.Uint32(uint32(playerId))}}
	sendOutgoing(&e, hash("swarm.EnterGame"))
}

func connectionRequestHandler(c chan ProtoRequest) {
	for {
		p := <-c
		request := swarm.ConnectionRequest{}
		if err := proto.Unmarshal(p.body, &request); err != nil {
			log.Println(err)
		}
		log.Println("Connection request")

		response := &swarm.ConnectionResponse{}
		sendProtoResponse(response, p.header.GetMethodHash())

		// check if the player wants to create a new game, or join an existing
		if request.GetCreateGame() {

			player := PlayerInfo{id: nextPlayerId}
			game := GameState{id: nextGameId}

			game.players = append(game.players, player)
			players[nextPlayerId] = player

			sendEnterGame(nextPlayerId, nextGameId, game.players)

			nextPlayerId++
			nextGameId++

		} else {

		}

	}
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

func parseProtoHeader(buf []byte) (err error, headerSize int16, header swarm.Header) {

	header = swarm.Header{}
	headerSize = 0

	// create reader, and parse header size
	reader := bytes.NewReader(buf)
	if err = binary.Read(reader, binary.BigEndian, &headerSize); err != nil {
		return
	}

	// parse header
	if err = proto.Unmarshal(buf[2:2+headerSize], &header); err != nil {
		return
	}

	return
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

	connectedClients[nextClientId] = ClientConnection{nextClientId, conn}
	nextClientId++

	// create channel for incoming messages, and spawn goroutine to process them
	incoming := make(chan []byte)
	go func(ch chan []byte) {
		// wait for packet
		_, buf, err := conn.ReadMessage()
		if err != nil {
			log.Print(err)
			return
		}
		ch <- buf
	}(incoming)

	for {

		// handle incoming and outgoing packets
		select {
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

		case req := <-responseChannel:
			log.Println("sending response")

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

			conn.WriteMessage(websocket.BinaryMessage, buf.Bytes())
		}
	}
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func initRequestHandlers() {
	c := make(chan ProtoRequest)
	requestChannels[hash("swarm.ConnectionRequest")] = c
	go connectionRequestHandler(c)
}

func main() {
	log.Println("** server started")
	initRequestHandlers()

	http.HandleFunc("/", handler)
	if err := http.ListenAndServe("localhost:8080", nil); err != nil {
		log.Fatal(err)
	}

}
