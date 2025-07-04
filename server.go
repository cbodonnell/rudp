package rudp

import (
	"errors"
	"net"
	"sync"
	"time"
)

// Server manages multiple UDP connections
type Server struct {
	mu          sync.RWMutex
	conn        *net.UDPConn
	connections map[string]*Connection

	// Events
	OnConnect    func(*Connection)
	OnDisconnect func(*Connection)
	OnMessage    func(*Connection, *Packet)

	done chan struct{}
}

// NewServer creates a new UDP server
func NewServer() *Server {
	return &Server{
		connections: make(map[string]*Connection),
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

			s.handlePacket(buffer[:n], addr)
		}
	}
}

// handlePacket routes a packet to the appropriate connection
func (s *Server) handlePacket(data []byte, addr *net.UDPAddr) {
	addrStr := addr.String()

	s.mu.Lock()
	conn, exists := s.connections[addrStr]
	if !exists {
		conn = NewConnection(s.conn, addr)
		s.connections[addrStr] = conn
		s.mu.Unlock()

		if s.OnConnect != nil {
			s.OnConnect(conn)
		}

		go s.handleConnection(conn)
	} else {
		s.mu.Unlock()
	}

	conn.HandleIncomingPacket(data)
}

// handleConnection processes packets from a specific connection
func (s *Server) handleConnection(conn *Connection) {
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
	delete(s.connections, conn.addr.String())
	s.mu.Unlock()

	if s.OnDisconnect != nil {
		s.OnDisconnect(conn)
	}
}

// cleanupConnections removes stale connections
func (s *Server) cleanupConnections() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			for addr, conn := range s.connections {
				if !conn.IsConnected() {
					conn.Close()
					delete(s.connections, addr)
				}
			}
			s.mu.Unlock()
		case <-s.done:
			return
		}
	}
}

// GetConnection returns a connection by address
func (s *Server) GetConnection(addr string) (*Connection, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conn, exists := s.connections[addr]
	return conn, exists
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
