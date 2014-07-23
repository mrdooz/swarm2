package main

import (
	"bytes"
	"code.google.com/p/goprotobuf/proto"
	"encoding/binary"
	"github.com/mrdooz/swarm2/protocol"
	"hash/fnv"
	"log"
)

func ParseProtoHeader(buf []byte) (err error, headerSize int16, header swarm.Header) {

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

func makeHash(str string) uint32 {
	h := Hash(str)
	_, n := methodHashToName[h]
	if !n {
		methodHashToName[h] = str
		log.Printf("hash: %s => %d\n", str, h)
	}
	return h
}

func Hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
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

func (state *GameState) toProtocol(p *swarm.GameState) {

	p = &swarm.GameState{GameId: proto.Uint32(state.gameId)}

	for _, player := range state.players {
		s := &swarm.PlayerState{}
		player.toProtocol(s)
		p.Players = append(p.Players, s)
	}
}
