package main

import (
	"fmt"
	"log"
	"time"

	"github.com/cbodonnell/rudp"
)

func main() {
	// Start server
	server := rudp.NewServer()

	server.OnConnect = func(conn *rudp.Connection) {
		fmt.Printf("Client connected: %s\n", conn.RemoteAddr())
	}

	server.OnMessage = func(conn *rudp.Connection, packet *rudp.Packet) {
		fmt.Printf("Server received: %s\n", string(packet.Data))
		// Echo back to client
		conn.Send([]byte("Echo: "+string(packet.Data)), rudp.Reliable)
	}

	server.OnDisconnect = func(conn *rudp.Connection) {
		fmt.Printf("Client disconnected: %s\n", conn.RemoteAddr())
	}

	if err := server.Listen(":8080"); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Server listening on :8080")

	// Start client
	go func() {
		time.Sleep(100 * time.Millisecond) // Let server start

		client := rudp.NewClient()

		client.OnMessage = func(packet *rudp.Packet) {
			fmt.Printf("Client received: %s\n", string(packet.Data))
		}

		client.OnDisconnect = func() {
			fmt.Println("Client disconnected")
		}

		if err := client.Connect("localhost:8080"); err != nil {
			log.Fatal(err)
		}

		fmt.Println("Client connected")

		// Send test messages
		messages := []string{"Hello", "World", "Reliable UDP"}
		for _, msg := range messages {
			client.Send([]byte(msg), rudp.Reliable)
			time.Sleep(50 * time.Millisecond)
		}

		time.Sleep(2 * time.Second)
		client.Close()
	}()

	// Run for demo
	time.Sleep(10 * time.Second)
	server.Close()
	fmt.Println("Demo complete")
}
