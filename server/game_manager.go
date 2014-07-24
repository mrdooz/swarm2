package main

import (
	"github.com/mrdooz/swarm2/protocol"
	"log"
	"time"
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

// TODO: consolidate these with playerstate/gamestate
type Player struct {
	chanState *chan *GameState
	state     *PlayerState
}

type Game struct {
	state   *GameState
	players []*Player
}

func (mgr *GameManager) addPlayerToGame(c *chan *GameState) (
	playerState *PlayerState, gameState *GameState) {

	// look for an existing game
	var game *Game

	// TODO: for now, just use the first game we find
	for _, g := range mgr.games {
		game = g
		gameState = g.state
		break
	}

	if game == nil {
		// create new game
		id := mgr.nextGameId
		gameState = &GameState{gameId: id}
		game = &Game{state: gameState}
		mgr.games[mgr.nextGameId] = game
		mgr.nextGameId++
	}

	// add the player to game, and the playestate to gamestate
	playerId := mgr.nextPlayerId
	playerState = &PlayerState{id: playerId}
	game.players = append(game.players, &Player{c, playerState})
	gameState.players = append(gameState.players, playerState)
	mgr.nextPlayerId++

	mgr.playerIdToGame[playerId] = game
	mgr.players[playerId] = playerState

	return
}

func (mgr *GameManager) run() {
	log.Println("[GM] run")

	mgr.gameService = GameService{make(chan *CreateGameRequest), nil, nil}
	mgr.playerState = make(chan swarm.PlayerState)
	mgr.games = make(map[uint32]*Game)
	mgr.players = make(map[uint32]*PlayerState)
	mgr.playerIdToGame = make(map[uint32]*Game)

	for {
		select {
		case req := <-mgr.gameService.createGameRequest:

			// save the game state channel for the player
			playerState, gameState := mgr.addPlayerToGame(&req.gameState)

			var players []uint32
			for _, p := range gameState.players {
				players = append(players, p.id)
			}

			req.response <- &CreateGameResponse{
				true,
				playerState.id,
				gameState.gameId,
				players}
			break

		case s := <-mgr.playerState:
			// look up the player/game
			playerId := s.GetId()
			player := mgr.players[playerId]

			// update the player state
			player.fromProtocol(&s)

			break

		case <-time.After(100 * time.Millisecond):
			// loop over all the games, and notify players of the game state
			for _, g := range mgr.games {
				for _, p := range g.players {
					*p.chanState <- g.state
				}
			}
			break
		}
	}
	log.Println("[GM] exit")

}
