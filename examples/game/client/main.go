package main

import (
	"encoding/json"
	"image/color"
	"log"

	"github.com/cbodonnell/rudp"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type MessageType string

const (
	MsgPlayerLogin      MessageType = "player_login"
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

type Game struct {
	client    *rudp.Client
	gameState *GameState
	playerID  string
}

func (g *Game) Update() error {
	if g.playerID == "" {
		// If player ID is not set, send login message
		loginMsg := Message{Type: MsgPlayerLogin}
		loginData, _ := json.Marshal(loginMsg)
		g.client.Send(loginData, rudp.Reliable)
		return nil
	}

	if g.client == nil || !g.client.IsConnected() {
		return nil
	}

	// Get current player
	player, exists := g.gameState.Players[g.playerID]
	if !exists {
		return nil
	}

	// Handle input
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		player.X -= 20
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		player.X += 20
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		player.Y -= 20
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		player.Y += 20
	}

	// Send movement to server
	playerData, _ := json.Marshal(player)
	msg := Message{Type: MsgPlayerMove, Data: playerData}
	msgData, _ := json.Marshal(msg)
	g.client.Send(msgData, rudp.Unreliable)

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	// Draw all players
	for _, player := range g.gameState.Players {
		playerColor := color.RGBA{255, 255, 255, 255} // white
		if player.ID == g.playerID {
			playerColor = color.RGBA{255, 100, 100, 255} // red
		}
		ebitenutil.DrawRect(screen, float64(player.X), float64(player.Y), 20, 20, playerColor)
	}

	ebitenutil.DebugPrint(screen, "Arrow keys to move\nYour ID: "+g.playerID)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return 320, 240
}

func main() {
	client := rudp.NewClient()
	game := &Game{
		client:    client,
		gameState: &GameState{Players: make(map[string]Player)},
	}

	client.OnMessage = func(packet *rudp.Packet) {
		var msg Message
		if err := json.Unmarshal(packet.Data, &msg); err != nil {
			return
		}

		switch msg.Type {
		case MsgPlayerAssignment:
			var assignment PlayerAssignment
			if err := json.Unmarshal(msg.Data, &assignment); err == nil {
				game.playerID = assignment.PlayerID
			}
		case MsgGameState:
			var state GameState
			if err := json.Unmarshal(msg.Data, &state); err == nil {
				game.gameState = &state
			}
		}
	}

	if err := client.Connect("localhost:8080"); err != nil {
		log.Fatal(err)
	}

	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("RUDP Game Demo")

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
