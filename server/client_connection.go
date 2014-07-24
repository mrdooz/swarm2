package main

import (
	"bytes"
	"code.google.com/p/goprotobuf/proto"
	"encoding/binary"
	"github.com/gorilla/websocket"
	"github.com/mrdooz/swarm2/protocol"
	"log"
	"time"
)

var (
	CONNECTION_REQUEST_METHOD_HASH  uint32 = Hash("swarm.ConnectionRequest")
	CONNECTION_RESPONSE_METHOD_HASH uint32 = Hash("swarm.ConnectionResponse")
	PING_REQUEST_METHOD_HASH        uint32 = Hash("swarm.PingRequest")
	PING_RESPONSE_METHOD_HASH       uint32 = Hash("swarm.PingResponse")
	PLAYER_STATE_METHOD_HASH        uint32 = Hash("swarm.PlayerState")

	methodHashToName map[uint32]string = make(map[uint32]string)
)

type ClientConnection struct {
	clientId     uint32
	token        uint32
	conn         *websocket.Conn
	outgoing     chan []byte
	disconnected chan bool
	gameService  GameService
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

	for {
		_, buf, err := conn.ReadMessage()
		if err != nil {
			client.disconnected <- true
			log.Println("[CC-R] Disconnected")
			break
		}

		err, headerSize, header := ParseProtoHeader(buf)
		if err != nil {
			log.Println("error parsing proto header")
		}
		//		log.Printf("recv: %d, %x", headerSize, header.GetMethodHash())

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
				client.sendProtoMessageResponse(
					&response,
					makeHash("swarm.ConnectionResponse"),
					header.GetToken())

				gameMgr.gameService.createGameRequest <- &CreateGameRequest{
					client.gameService.createGameResponse,
					client.gameService.gameState,
					request.GetCreateGame()}
				break

			case PLAYER_STATE_METHOD_HASH:
				state := swarm.PlayerState{}
				if err := proto.Unmarshal(body, &state); err != nil {
					log.Println(err)
				} else {
					gameMgr.playerState <- state
				}
				break
			}
		}
	}

}

func (client *ClientConnection) run() {

	log.Println("[CC] run")

	client.gameService = GameService{
		nil,
		make(chan *CreateGameResponse),
		make(chan *GameState)}
	client.outgoing = make(chan []byte)
	client.disconnected = make(chan bool)

	go client.writer()
	go client.reader()

	done := false
	for !done {
		select {
		case response := <-client.gameService.createGameResponse:
			e := swarm.EnterGame{
				GameId:   &response.gameId,
				PlayerId: &response.playerId,
				Players:  response.players}

			client.sendProtoMessageRequest(&e, makeHash("swarm.EnterGame"))
			break

		case <-time.After(10 * time.Second):
			client.sendProtoMessageRequest(&swarm.PingRequest{}, makeHash("swarm.PingRequest"))
			break

		case gameState := <-client.gameService.gameState:
			client.sendProtoMessageRequest(gameState.toProtocol(), makeHash("swarm.GameState"))
			break

		case <-client.disconnected:
			done = true
			break
		}

	}

	hub.clientDisconnected <- client.clientId
	log.Println("[CC] exit")
}

func (client *ClientConnection) sendProtoMessageRequest(
	message ProtobufMessage,
	methodHash uint32) bool {

	res := client.sendProtoMessageInner(message, methodHash, client.token, false)
	if res {
		client.token++
	}
	return true
}

func (client *ClientConnection) sendProtoMessageResponse(
	message ProtobufMessage,
	methodHash uint32,
	token uint32) bool {

	return client.sendProtoMessageInner(message, methodHash, token, true)
}

func (client *ClientConnection) sendProtoMessageInner(
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
		MethodHash: &methodHash,
		Token:      &token,
		IsResponse: &isResponse,
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

	client.outgoing <- buf.Bytes()

	return true
}
