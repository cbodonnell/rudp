package main

import (
	"encoding/json"
	"image/color"
	"log"

	"github.com/cbodonnell/rudp"
	"github.com/cbodonnell/rudp/examples/game/pkg/types"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type Game struct {
	client    *rudp.Client
	gameState *types.GameState
	playerID  string
}

func (g *Game) Update() error {
	if g.client == nil || !g.client.IsConnected() {
		return nil
	}

	// Handle input
	input := types.PlayerInput{}
	if ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		input.X -= 1
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) || ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		input.X += 1
	}
	if ebiten.IsKeyPressed(ebiten.KeyW) || ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		input.Y -= 1
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) || ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		input.Y += 1
	}

	// Send movement to server
	inputData, err := json.Marshal(input)
	if err != nil {
		return err
	}
	msg := types.Message{Type: types.MsgClientPlayerInput, Data: inputData}
	msgData, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if err := g.client.Send(msgData, rudp.Unreliable); err != nil {
		return err
	}

	// TODO: Client prediction and reconciliation
	// player.X += input.X
	// player.Y += input.Y

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	// Sort players by ID with the local player first
	var localPlayer *types.Player
	remotePlayers := make([]*types.Player, 0, len(g.gameState.Players))
	for _, player := range g.gameState.Players {
		if player.ID == g.playerID {
			localPlayer = player
		} else {
			remotePlayers = append(remotePlayers, player)
		}
	}
	// Draw remote players
	for _, player := range remotePlayers {
		playerColor := color.RGBA{255, 255, 255, 255} // white
		ebitenutil.DrawRect(screen, float64(player.X), float64(player.Y), 20, 20, playerColor)
	}
	// Draw local player
	if localPlayer != nil {
		playerColor := color.RGBA{100, 255, 100, 255} // green
		ebitenutil.DrawRect(screen, float64(localPlayer.X), float64(localPlayer.Y), 20, 20, playerColor)
	}

	ebitenutil.DebugPrint(screen, "WASD or Arrow keys to move\nYour ID: "+g.playerID)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return 320, 240
}

func main() {
	client := rudp.NewClient()
	game := &Game{
		client:    client,
		gameState: &types.GameState{Players: make(map[string]*types.Player)},
	}

	client.OnMessage = func(packet *rudp.Packet) {
		var msg types.Message
		if err := json.Unmarshal(packet.Data, &msg); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			return
		}

		switch msg.Type {
		case types.MsgServerPlayerAssignment:
			var assignment types.PlayerAssignment
			if err := json.Unmarshal(msg.Data, &assignment); err != nil {
				log.Printf("Failed to unmarshal player assignment: %v", err)
				return
			}
			game.playerID = assignment.PlayerID
		case types.MsgServerGameState:
			var state types.GameState
			if err := json.Unmarshal(msg.Data, &state); err != nil {
				log.Printf("Failed to unmarshal game state: %v", err)
				return
			}
			game.gameState = &state
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
