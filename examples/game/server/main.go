package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/cbodonnell/rudp"
	"github.com/cbodonnell/rudp/examples/game/pkg/types"
)

type ConnectionEvent struct {
	Conn *rudp.Connection
	Type ConnectionEventType
}

type ConnectionEventType string

const (
	ConnectionEventConnect    ConnectionEventType = "connect"
	ConnectionEventDisconnect ConnectionEventType = "disconnect"
)

type ClientMessage struct {
	Conn   *rudp.Connection
	Packet *rudp.Packet
}

func generatePlayerID() string {
	return fmt.Sprintf("player_%d", rand.Intn(10000))
}

func main() {
	server := rudp.NewServer()
	gameState := &types.GameState{Players: make(map[string]*types.Player)}
	connToPlayer := make(map[string]string) // conn addr -> player ID

	connEventCh := make(chan ConnectionEvent, 1000)
	clientMsgCh := make(chan ClientMessage, 1000)

	server.OnConnect = func(conn *rudp.Connection) {
		connEventCh <- ConnectionEvent{Conn: conn, Type: ConnectionEventConnect}
	}

	server.OnMessage = func(conn *rudp.Connection, packet *rudp.Packet) {
		clientMsgCh <- ClientMessage{Conn: conn, Packet: packet}
	}

	server.OnDisconnect = func(conn *rudp.Connection) {
		connEventCh <- ConnectionEvent{Conn: conn, Type: ConnectionEventDisconnect}
	}

	fmt.Println("Game server starting on :8080")
	if err := server.Listen(":8080"); err != nil {
		log.Fatal(err)
	}

	// Start the game loop
	gameLoopTicker := time.NewTicker(50 * time.Millisecond)
	defer gameLoopTicker.Stop()
	for range gameLoopTicker.C {
		// first process connection events
		for len(connEventCh) > 0 {
			event := <-connEventCh
			switch event.Type {
			case ConnectionEventConnect:
				conn := event.Conn
				playerID := generatePlayerID()
				connAddr := conn.RemoteAddr().String()
				connToPlayer[connAddr] = playerID
				p := &types.Player{ID: playerID, X: 100, Y: 100}
				gameState.Players[playerID] = p

				fmt.Printf("Player joined: %s (ID: %s)\n", connAddr, playerID)

				// Send player assignment
				assignment := types.PlayerAssignment{PlayerID: playerID}
				assignmentData, err := json.Marshal(assignment)
				if err != nil {
					log.Printf("Failed to marshal player assignment: %v", err)
					continue
				}
				msg := types.Message{Type: types.MsgServerPlayerAssignment, Data: assignmentData}
				msgData, err := json.Marshal(msg)
				if err != nil {
					log.Printf("Failed to marshal player assignment: %v", err)
					continue
				}
				if err := conn.Send(msgData, rudp.Reliable); err != nil {
					log.Printf("Failed to send player assignment: %v", err)
					continue
				}

				// Send initial game state
				stateData, err := json.Marshal(gameState)
				if err != nil {
					log.Printf("Failed to marshal game state: %v", err)
					continue
				}
				stateMsg := types.Message{Type: types.MsgServerGameState, Data: stateData}
				stateMsgData, err := json.Marshal(stateMsg)
				if err != nil {
					log.Printf("Failed to marshal game state message: %v", err)
					continue
				}
				if err := conn.Send(stateMsgData, rudp.Reliable); err != nil {
					log.Printf("Failed to send game state: %v", err)
					continue
				}
			case ConnectionEventDisconnect:
				conn := event.Conn
				connAddr := conn.RemoteAddr().String()
				if playerID, exists := connToPlayer[connAddr]; exists {
					delete(gameState.Players, playerID)
					delete(connToPlayer, connAddr)
					fmt.Printf("Player left: %s (ID: %s)\n", connAddr, playerID)
				}
			default:
				log.Printf("Unknown connection event: %s", event.Type)
			}
		}

		// then process client messages
		for len(clientMsgCh) > 0 {
			cm := <-clientMsgCh
			connAddr := cm.Conn.RemoteAddr().String()
			playerID, exists := connToPlayer[connAddr]
			if !exists {
				log.Printf("Received message from unknown player: %s", connAddr)
				continue
			}
			player, exists := gameState.Players[playerID]
			if !exists {
				log.Printf("Player not found in game state: %s (ID: %s)", connAddr, playerID)
				continue
			}

			packet := cm.Packet
			var msg types.Message
			if err := json.Unmarshal(packet.Data, &msg); err != nil {
				log.Printf("Failed to unmarshal message: %v", err)
				continue
			}
			switch msg.Type {
			case types.MsgClientPlayerLogin:
				// TODO: not implemented yet
			case types.MsgClientPlayerLogout:
				conn := cm.Conn
				connAddr := conn.RemoteAddr().String()
				if playerID, exists := connToPlayer[connAddr]; exists {
					delete(gameState.Players, playerID)
					delete(connToPlayer, connAddr)
					fmt.Printf("Player left: %s (ID: %s)\n", connAddr, playerID)
				}
			case types.MsgClientPlayerInput:
				var input types.PlayerInput
				if err := json.Unmarshal(msg.Data, &input); err != nil {
					log.Printf("Failed to unmarshal player input: %v", err)
					continue
				}

				player.X += input.X
				player.Y += input.Y
			default:
				log.Printf("Unknown message type from %s: %s", connAddr, msg.Type)
				continue
			}
		}

		// lastly, broadcast the game state to all players
		stateData, err := json.Marshal(gameState)
		if err != nil {
			log.Printf("Failed to marshal game state: %v", err)
			continue
		}
		stateMsg := types.Message{Type: types.MsgServerGameState, Data: stateData}
		stateMsgData, err := json.Marshal(stateMsg)
		if err != nil {
			log.Printf("Failed to marshal game state message: %v", err)
			continue
		}
		if err := server.Broadcast(stateMsgData, rudp.Unreliable); err != nil {
			log.Printf("Failed to broadcast game state: %v", err)
			continue
		}
	}
}
