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
	playerLeft  chan uint32

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

func (mgr *GameManager) removePlayerFromGame(playerId uint32) {

	log.Printf("[GM] removing player: %d", playerId)

	// check that the player exists
	_, playerExists := mgr.playerIdToGame[playerId]
	if !playerExists {
		return
	}

	game := mgr.playerIdToGame[playerId]
	gameState := game.state

	// remove player from player maps
	delete(mgr.players, playerId)
	delete(mgr.playerIdToGame, playerId)

	// remove player from game state
	for i, e := range game.players {
		if e.state.id == playerId {
			// player found, so delete it
			game.players = append(game.players[:i], game.players[i+1:]...)
			break
		}
	}

	for i, e := range gameState.players {
		if e.id == playerId {
			gameState.players = append(gameState.players[:i], gameState.players[i+1:]...)
			break
		}
	}

	log.Printf("[GM] removed player: %d", playerId)

	// delete the game if this was the last player
	if len(gameState.players) == 0 {
		gameId := gameState.gameId
		delete(mgr.games, gameId)
		log.Printf("[GM] ended game: %d", gameId)

	}

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

		log.Printf("[GM] created game: %d", id)
	}

	// add the player to game, and the playestate to gamestate
	playerId := mgr.nextPlayerId
	playerState = &PlayerState{id: playerId}
	game.players = append(game.players, &Player{c, playerState})
	gameState.players = append(gameState.players, playerState)
	mgr.nextPlayerId++

	mgr.playerIdToGame[playerId] = game
	mgr.players[playerId] = playerState

	log.Printf("[GM] added player: %d", playerId)

	return
}

func (mgr *GameManager) run() {
	log.Println("[GM] run")

	mgr.nextGameId = 1
	mgr.nextPlayerId = 1

	mgr.gameService = GameService{make(chan *CreateGameRequest), nil, nil}
	mgr.playerState = make(chan swarm.PlayerState)
	mgr.playerLeft = make(chan uint32)
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

		case playerId := <-mgr.playerLeft:
			mgr.removePlayerFromGame(playerId)
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
