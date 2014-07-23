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

type ClientConnection struct {
	clientId     uint32
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
	outgoing := client.outgoing

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
					nil,
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
	hub.clientDisconnected <- client.clientId
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
