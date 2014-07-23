package main

type ProtobufMessage interface {
	ProtoMessage()
	Reset()
	String() string
}

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

type GameState struct {
	gameId  uint32
	players []PlayerState
}
