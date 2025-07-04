package rudp

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"net"
	"sync"
	"time"
)

// Server manages multiple UDP connections
type Server struct {
	mu          sync.RWMutex
	conn        *net.UDPConn
	connections map[uint32]*Connection

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
	hash := addrHash(addr)

	s.mu.Lock()
	conn, exists := s.connections[hash]
	if !exists {
		conn = NewConnection(s.conn, addr)
		s.connections[hash] = conn
		s.mu.Unlock()

		if s.OnConnect != nil {
			s.OnConnect(conn)
		}

		go s.handleConnection(hash, conn)
	} else {
		s.mu.Unlock()
	}

	return conn.HandleIncomingPacket(data)
}

// handleConnection processes packets from a specific connection
func (s *Server) handleConnection(hash uint32, conn *Connection) {
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
	delete(s.connections, hash)
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
func (s *Server) GetConnection(hash uint32) (*Connection, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conn, exists := s.connections[hash]
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

// addrHash generates a hash for the UDP address
func addrHash(addr *net.UDPAddr) uint32 {
	port := make([]byte, 4)
	binary.LittleEndian.PutUint32(port, uint32(addr.Port))
	return crc32.ChecksumIEEE(append(addr.IP, port...))
}
