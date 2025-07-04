# RUDP - Reliable UDP for Go

A reliable UDP networking library designed for multiplayer games and real-time applications.

## Features

- **Multiple Delivery Modes**: Unreliable, UnreliableOrdered, Reliable, ReliableOrdered
- **Automatic Retransmission**: Configurable timeout and retry limits
- **Connection Management**: Auto-cleanup of stale connections

## Quick Start

### Server
```go
server := rudp.NewServer()
server.OnMessage = func(conn *rudp.Connection, packet *rudp.Packet) {
    // Echo back to client
    conn.Send(packet.Data, rudp.Reliable)
}
server.Listen(":8080")
```

### Client
```go
client := rudp.NewClient()
client.OnMessage = func(packet *rudp.Packet) {
    fmt.Printf("Received: %s\n", packet.Data)
}
client.Connect("localhost:8080")
client.Send([]byte("Hello World"), rudp.Reliable)
```

## Examples

- **Basic**: Simple echo server ([examples/basic](examples/basic))
- **Game**: Real-time multiplayer demo with Ebiten ([examples/game](examples/game))

## Installation

```bash
go get github.com/cbodonnell/rudp
```

## Configuration

*TODO: make these configurable variables*

```go
const (
    MaxPacketSize         = 1400              // Maximum UDP packet size
    RetransmissionTimeout = 100 * time.Millisecond
    MaxRetransmissions    = 5
    InactivityTimeout     = 30 * time.Second
)
```
