package main

import (
	"github.com/mrdooz/swarm2/protocol"
)

type GameManager struct {
	nextGameId   uint32
	nextPlayerId uint32

	games          map[uint32]*Game
	players        map[uint32]*PlayerState
	playerIdToGame map[uint32]*Game

	playerState chan swarm.PlayerState

	gameService GameService
}

type Player struct {
	chanState *chan GameState
	state     *PlayerState
}

type Game struct {
	state   *GameState
	players []*Player
}

func (mgr *GameManager) addPlayerToGame(c *chan GameState) *PlayerState {

	// look for an existing game
	var game *Game = nil
	for _, g := range mgr.games {
		game = g
		break
	}

	if game == nil {
		// create new game
		id := mgr.nextGameId
		game = &Game{state: &GameState{gameId: id}}
		mgr.games[mgr.nextGameId] = game
		mgr.nextGameId++
	}

	// add player to game
	playerId := mgr.nextPlayerId
	state := &PlayerState{id: playerId}
	game.players = append(game.players, &Player{c, state})
	mgr.nextPlayerId++

	mgr.playerIdToGame[playerId] = game
	mgr.players[playerId] = state

	return state
}

func (mgr *GameManager) run() {

	mgr.gameService = GameService{make(chan CreateGameRequest), nil, nil}
	mgr.playerState = make(chan swarm.PlayerState)

	select {
	case req := <-mgr.gameService.createGameRequest:

		// save the game state channel for the player
		p := mgr.addPlayerToGame(req.gameState)

		*req.response <- CreateGameResponse{
			true,
			p.id,
			0,
			[]uint32{}}
		mgr.nextGameId++
		mgr.nextPlayerId++
		break

	case s := <-mgr.playerState:
		// look up the player/game
		playerId := s.GetId()
		game := mgr.playerIdToGame[playerId]
		player := mgr.players[playerId]

		// update the player state
		player.fromProtocol(&s)

		break

		// case <-time.After(100 * time.Millisecond):
		// 	for _, g := range mgr.games {

		// 	}
		// 	break
	}
}
