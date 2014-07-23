package main

import (
	"github.com/mrdooz/swarm2/protocol"
)

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

		// save the game state channel for the player
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
