package server_utils

import (
	"bytes"
	"code.google.com/p/goprotobuf/proto"
	"encoding/binary"
	"github.com/mrdooz/swarm2/protocol"
	"hash/fnv"
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

func Hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
