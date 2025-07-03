package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/cbodonnell/rudp"
)

type MessageType string

const (
	MsgPlayerAssignment MessageType = "player_assignment"
	MsgPlayerMove       MessageType = "player_move"
	MsgGameState        MessageType = "game_state"
)

type Message struct {
	Type MessageType     `json:"type"`
	Data json.RawMessage `json:"data"`
}

type Player struct {
	ID string `json:"id"`
	X  int    `json:"x"`
	Y  int    `json:"y"`
}

type PlayerAssignment struct {
	PlayerID string `json:"player_id"`
}

type GameState struct {
	Players map[string]Player `json:"players"`
}

func generatePlayerID() string {
	return fmt.Sprintf("player_%d", rand.Intn(10000))
}

func main() {
	rand.Seed(time.Now().UnixNano())
	server := rudp.NewServer()
	gameState := &GameState{Players: make(map[string]Player)}
	connToPlayer := make(map[string]string) // conn addr -> player ID

	server.OnConnect = func(conn *rudp.Connection) {
		playerID := generatePlayerID()
		connAddr := conn.RemoteAddr().String()
		connToPlayer[connAddr] = playerID
		gameState.Players[playerID] = Player{ID: playerID, X: 100, Y: 100}

		fmt.Printf("Player joined: %s (ID: %s)\n", connAddr, playerID)

		// Send player assignment
		assignment := PlayerAssignment{PlayerID: playerID}
		assignmentData, _ := json.Marshal(assignment)
		msg := Message{Type: MsgPlayerAssignment, Data: assignmentData}
		msgData, _ := json.Marshal(msg)
		conn.Send(msgData, rudp.Reliable)

		// Send initial game state
		stateData, _ := json.Marshal(gameState)
		stateMsg := Message{Type: MsgGameState, Data: stateData}
		stateMsgData, _ := json.Marshal(stateMsg)
		conn.Send(stateMsgData, rudp.Reliable)
	}

	server.OnMessage = func(conn *rudp.Connection, packet *rudp.Packet) {
		var msg Message
		if err := json.Unmarshal(packet.Data, &msg); err != nil {
			return
		}

		switch msg.Type {
		case MsgPlayerMove:
			var player Player
			if err := json.Unmarshal(msg.Data, &player); err != nil {
				return
			}

			// Update player position
			gameState.Players[player.ID] = player

			// Broadcast to all players
			stateData, _ := json.Marshal(gameState)
			stateMsg := Message{Type: MsgGameState, Data: stateData}
			stateMsgData, _ := json.Marshal(stateMsg)
			server.Broadcast(stateMsgData, rudp.Unreliable)
		}
	}

	server.OnDisconnect = func(conn *rudp.Connection) {
		connAddr := conn.RemoteAddr().String()
		if playerID, exists := connToPlayer[connAddr]; exists {
			delete(gameState.Players, playerID)
			delete(connToPlayer, connAddr)
			fmt.Printf("Player left: %s (ID: %s)\n", connAddr, playerID)
		}
	}

	fmt.Println("Game server starting on :8080")
	if err := server.Listen(":8080"); err != nil {
		log.Fatal(err)
	}

	select {} // Keep running
}
