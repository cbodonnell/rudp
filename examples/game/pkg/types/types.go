package types

import "encoding/json"

type MessageType string

const (
	MsgServerPlayerAssignment MessageType = "player_assignment"
	MsgServerGameState        MessageType = "game_state"

	MsgClientPlayerLogin  MessageType = "player_login"  // TODO: implement login
	MsgClientPlayerLogout MessageType = "player_logout" // TODO: implement logout
	MsgClientPlayerInput  MessageType = "player_move"
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

type PlayerInput struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type PlayerAssignment struct {
	PlayerID string `json:"player_id"`
}

type GameState struct {
	Players map[string]*Player `json:"players"`
}
