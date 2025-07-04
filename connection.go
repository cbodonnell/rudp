package rudp

import (
	"net"
	"sync"
	"time"
)

const (
	InactivityTimeout = 30 * time.Second // Timeout for connection inactivity
)

// Connection represents a reliable UDP connection to a peer
type Connection struct {
	mu   sync.RWMutex
	addr *net.UDPAddr
	conn *net.UDPConn

	// Sequence tracking
	localSequence  uint16
	remoteSequence uint16
	ackBits        uint32

	// Reliability
	pendingAcks   map[uint16]*Packet
	recvBuffer    map[uint16]*Packet
	orderedBuffer map[uint16]*Packet

	// State
	lastReceived time.Time
	lastSent     time.Time
	closed       bool

	// Channels
	inbound  chan *Packet
	outbound chan *Packet
	done     chan struct{}
}

// NewConnection creates a new connection to the specified address
func NewConnection(conn *net.UDPConn, addr *net.UDPAddr) *Connection {
	c := &Connection{
		addr:          addr,
		conn:          conn,
		pendingAcks:   make(map[uint16]*Packet),
		recvBuffer:    make(map[uint16]*Packet),
		orderedBuffer: make(map[uint16]*Packet),
		lastReceived:  time.Now(),
		inbound:       make(chan *Packet, 256),
		outbound:      make(chan *Packet, 256),
		done:          make(chan struct{}),
	}

	go c.processOutbound()
	go c.processRetransmissions()

	return c
}

// Send queues a packet for transmission
func (c *Connection) Send(data []byte, mode DeliveryMode) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return ErrConnectionClosed
	}

	if len(data) > MaxPacketSize-HeaderSize {
		return ErrPacketTooLarge
	}

	packet := &Packet{
		Sequence:  c.localSequence,
		Ack:       c.remoteSequence,
		AckBits:   c.ackBits,
		Mode:      mode,
		Data:      data,
		Timestamp: time.Now().UnixNano(),
	}

	c.localSequence++

	if packet.IsReliable() {
		c.pendingAcks[packet.Sequence] = packet
	}

	select {
	case c.outbound <- packet:
		return nil
	default:
		return ErrBufferFull
	}
}

// Receive returns the next available packet
func (c *Connection) Receive() (*Packet, error) {
	select {
	case packet := <-c.inbound:
		return packet, nil
	case <-c.done:
		return nil, ErrConnectionClosed
	}
}

// Close closes the connection
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	close(c.done)
	return nil
}

// IsConnected returns true if the connection is active
func (c *Connection) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return !c.closed && time.Since(c.lastReceived) < InactivityTimeout
}

// RemoteAddr returns the remote address of the connection
func (c *Connection) RemoteAddr() *net.UDPAddr {
	return c.addr
}
