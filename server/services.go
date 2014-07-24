package main

type GameService struct {
	createGameRequest  chan *CreateGameRequest
	createGameResponse chan *CreateGameResponse
	gameState          chan *GameState
}

type CreateGameRequest struct {
	response   chan *CreateGameResponse
	gameState  chan *GameState
	createGame bool
}

type CreateGameResponse struct {
	newGame  bool
	playerId uint32
	gameId   uint32
	players  []uint32
}
