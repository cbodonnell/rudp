package rudp

import (
	"net"
	"time"
)

// Client represents a UDP client connection
type Client struct {
	conn       *net.UDPConn
	connection *Connection

	// Events
	OnMessage    func(*Packet)
	OnDisconnect func()

	done chan struct{}
}

// NewClient creates a new UDP client
func NewClient() *Client {
	return &Client{
		done: make(chan struct{}),
	}
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

	c.connection = NewConnection(c.conn, serverAddr)

	go c.handlePackets()
	go c.handleConnection()

	return nil
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

			c.connection.HandleIncomingPacket(buffer[:n])
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
	return c.connection != nil && c.connection.IsConnected()
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
