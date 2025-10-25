package rudp

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

// Client represents a UDP client connection
type Client struct {
	conn       *net.UDPConn
	connection *Connection
	clientID   uint32
	connected  bool

	// Events
	OnMessage    func(*Packet)
	OnDisconnect func()

	done chan struct{}
}

// NewClient creates a new UDP client
func NewClient() *Client {
	return &Client{
		clientID: generateClientID(),
		done:     make(chan struct{}),
	}
}

// generateClientID creates a random 32-bit client identifier
func generateClientID() uint32 {
	b := make([]byte, 4)
	rand.Read(b)
	return binary.LittleEndian.Uint32(b)
}

// Connect establishes a connection to the server
func (c *Client) Connect(addr string) error {
	serverAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}

	c.conn, err = net.ListenUDP("udp", nil)
	if err != nil {
		return err
	}

	c.connection = NewConnection(c.conn, serverAddr, c.clientID)

	// Perform handshake BEFORE starting background goroutines
	if err := c.performHandshake(); err != nil {
		c.Close()
		return err
	}

	// Now start packet processing
	go c.handlePackets()
	go c.handleConnection()

	return nil
}

// performHandshake sends CONNECT and waits for CONNECT_ACK
func (c *Client) performHandshake() error {
	connectPacket := &Packet{
		Type:     CONNECT,
		ClientID: c.clientID,
		Data:     []byte{},
	}

	// Send CONNECT packet
	data := connectPacket.Marshal()
	_, err := c.conn.WriteToUDP(data, c.connection.RemoteAddr())
	if err != nil {
		return err
	}

	// Wait for CONNECT_ACK with timeout (no other goroutines reading yet)
	buffer := make([]byte, MaxPacketSize)
	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		c.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, _, err := c.conn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return err
		}

		packet := &Packet{}
		if err := packet.Unmarshal(buffer[:n]); err != nil {
			fmt.Printf("Failed to unmarshal packet during handshake: %v\n", err)
			continue
		}

		if packet.Type == CONNECT_ACK && packet.ClientID == c.clientID {
			c.connected = true
			fmt.Printf("Connected to server with clientID: %d\n", c.clientID)
			return nil
		}
	}

	return fmt.Errorf("handshake timeout: no CONNECT_ACK received")
}

// handlePackets reads incoming UDP packets
func (c *Client) handlePackets() {
	buffer := make([]byte, MaxPacketSize)

	for {
		select {
		case <-c.done:
			return
		default:
			c.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, err := c.conn.Read(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return
			}

			// Parse packet to check type
			packet := &Packet{}
			if err := packet.Unmarshal(buffer[:n]); err != nil {
				continue
			}

			// Ignore handshake packets (already handled during Connect)
			if packet.Type == CONNECT || packet.Type == CONNECT_ACK {
				continue
			}

			if err := c.connection.HandleIncomingPacket(packet); err != nil {
				// TODO: log the error
				continue
			}
		}
	}
}

// handleConnection processes packets from the server
func (c *Client) handleConnection() {
	for {
		packet, err := c.connection.Receive()
		if err != nil {
			break
		}

		if c.OnMessage != nil {
			c.OnMessage(packet)
		}
	}

	// Connection closed
	if c.OnDisconnect != nil {
		c.OnDisconnect()
	}
}

// Send transmits data to the server
func (c *Client) Send(data []byte, mode DeliveryMode) error {
	if c.connection == nil {
		return ErrConnectionClosed
	}
	return c.connection.Send(data, mode)
}

// IsConnected returns true if connected to server
func (c *Client) IsConnected() bool {
	return c.connected && c.connection != nil && c.connection.IsConnected()
}

// ClientID returns the client's unique identifier
func (c *Client) ClientID() uint32 {
	return c.clientID
}

// Close disconnects from the server
func (c *Client) Close() error {
	close(c.done)

	if c.connection != nil {
		c.connection.Close()
	}

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// RemoteAddr returns the server address
func (c *Client) RemoteAddr() net.Addr {
	if c.connection != nil {
		return c.connection.RemoteAddr()
	}
	return nil
}
