package rudp

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// Server manages multiple UDP connections
type Server struct {
	mu          sync.RWMutex
	conn        *net.UDPConn
	connections map[uint32]*Connection // Keyed by ClientID

	// Events
	OnConnect    func(*Connection)
	OnDisconnect func(*Connection)
	OnMessage    func(*Connection, *Packet)

	done chan struct{}
}

// NewServer creates a new UDP server
func NewServer() *Server {
	return &Server{
		connections: make(map[uint32]*Connection),
		done:        make(chan struct{}),
	}
}

// Listen starts the server on the specified address
func (s *Server) Listen(addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}

	s.conn, err = net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}

	go s.handlePackets()
	go s.cleanupConnections()

	return nil
}

// handlePackets processes incoming UDP packets
func (s *Server) handlePackets() {
	buffer := make([]byte, MaxPacketSize)

	for {
		select {
		case <-s.done:
			return
		default:
			n, addr, err := s.conn.ReadFromUDP(buffer)
			if err != nil {
				continue
			}

			if err := s.handlePacket(buffer[:n], addr); err != nil {
				// TODO: log the error
				continue
			}
		}
	}
}

// handlePacket routes a packet to the appropriate connection
func (s *Server) handlePacket(data []byte, addr *net.UDPAddr) error {
	// Parse packet to determine type and client ID
	packet := &Packet{}
	if err := packet.Unmarshal(data); err != nil {
		return err
	}

	// Handle CONNECT packets specially
	if packet.Type == CONNECT {
		return s.handleConnect(packet, addr)
	}

	// Route to connection by ClientID (O(1) lookup)
	s.mu.RLock()
	conn, exists := s.connections[packet.ClientID]
	s.mu.RUnlock()

	if !exists {
		// Silently drop packets from unknown clients
		// They need to send CONNECT first
		return nil
	}

	// Update address if client reconnected from different port/IP
	if conn.RemoteAddr().String() != addr.String() {
		fmt.Printf("Client %d address changed: %s -> %s\n",
			packet.ClientID, conn.RemoteAddr().String(), addr.String())
		conn.UpdateAddr(addr)
	}

	return conn.HandleIncomingPacket(packet)
}

// handleConnect processes CONNECT packets and establishes new connections
func (s *Server) handleConnect(packet *Packet, addr *net.UDPAddr) error {
	clientID := packet.ClientID

	s.mu.Lock()
	conn, exists := s.connections[clientID]

	if exists {
		// Client reconnecting from different address
		if conn.RemoteAddr().String() != addr.String() {
			fmt.Printf("Client %d reconnecting from new addr: %s (was: %s)\n",
				clientID, addr.String(), conn.RemoteAddr().String())
			conn.UpdateAddr(addr)
		}
		s.mu.Unlock()
	} else {
		// New connection
		fmt.Printf("New connection from addr: %s, clientID: %d\n", addr.String(), clientID)
		conn = NewConnection(s.conn, addr, clientID)
		s.connections[clientID] = conn
		s.mu.Unlock()

		if s.OnConnect != nil {
			s.OnConnect(conn)
		}

		go s.handleConnection(clientID, conn)
	}

	// Send CONNECT_ACK
	ackPacket := &Packet{
		Type:     CONNECT_ACK,
		ClientID: clientID,
		Data:     []byte{},
	}
	ackData := ackPacket.Marshal()
	s.conn.WriteToUDP(ackData, addr)

	return nil
}

// handleConnection processes packets from a specific connection
func (s *Server) handleConnection(clientID uint32, conn *Connection) {
	for {
		packet, err := conn.Receive()
		if err != nil {
			break
		}

		if s.OnMessage != nil {
			s.OnMessage(conn, packet)
		}
	}

	// Connection closed
	s.mu.Lock()
	delete(s.connections, clientID)
	s.mu.Unlock()

	if s.OnDisconnect != nil {
		s.OnDisconnect(conn)
	}
}

// cleanupConnections removes stale connections
func (s *Server) cleanupConnections() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			for clientID, conn := range s.connections {
				if !conn.IsConnected() {
					conn.Close()
					delete(s.connections, clientID)
				}
			}
			s.mu.Unlock()
		case <-s.done:
			return
		}
	}
}

// Broadcast sends a packet to all connected clients
func (s *Server) Broadcast(data []byte, mode DeliveryMode) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	errs := make([]error, 0)
	for _, conn := range s.connections {
		if err := conn.Send(data, mode); err != nil {
			errs = append(errs, err)
			continue
		}
	}

	return errors.Join(errs...)
}

// Close shuts down the server
func (s *Server) Close() error {
	close(s.done)

	s.mu.Lock()
	for _, conn := range s.connections {
		conn.Close()
	}
	s.mu.Unlock()

	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
