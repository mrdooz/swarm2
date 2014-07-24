package main

import (
	"github.com/gorilla/websocket"
	"log"
)

type Hub struct {
	clientConnected    chan *websocket.Conn
	clientDisconnected chan uint32

	nextClientId uint32
	clients      map[uint32]*ClientConnection
}

func (mgr *Hub) run() {

	log.Println("[HUB] run")

	mgr.clientConnected = make(chan *websocket.Conn)
	mgr.clientDisconnected = make(chan uint32)
	mgr.clients = make(map[uint32]*ClientConnection)

	for {
		select {
		case conn := <-mgr.clientConnected:
			log.Println("[HUB] Client connected")
			// client connected, update structs and create the goroutine to
			// serve it
			clientId := mgr.nextClientId
			client := &ClientConnection{clientId: clientId, conn: conn}

			mgr.clients[clientId] = client
			mgr.nextClientId++
			go client.run()
			break

		case clientId := <-mgr.clientDisconnected:
			log.Println("[HUB] Client disconnected")
			delete(mgr.clients, clientId)
			break
		}
	}

	log.Println("[HUB] exit")
}
