package rudp

import (
	"encoding/binary"
	"time"
)

// DeliveryMode defines how packets should be delivered
type DeliveryMode byte

const (
	Unreliable DeliveryMode = iota
	UnreliableOrdered
	Reliable
	ReliableOrdered
)

const MaxPacketSize = 1400 // bytes

// Packet represents a network packet with metadata
type Packet struct {
	ID        uint16
	Sequence  uint16
	Ack       uint16
	AckBits   uint32
	Mode      DeliveryMode
	Timestamp int64
	Data      []byte
	Attempts  int
	LastSent  time.Time
}

const HeaderSize = 11 // bytes

// Marshal serializes the packet for network transmission
func (p *Packet) Marshal() []byte {
	buf := make([]byte, HeaderSize+len(p.Data))

	binary.LittleEndian.PutUint16(buf[0:2], p.Sequence)
	binary.LittleEndian.PutUint16(buf[2:4], p.Ack)
	binary.LittleEndian.PutUint32(buf[4:8], p.AckBits)
	buf[8] = byte(p.Mode)
	binary.LittleEndian.PutUint16(buf[9:11], uint16(len(p.Data)))

	copy(buf[HeaderSize:], p.Data)
	return buf
}

// Unmarshal deserializes a packet from network data
func (p *Packet) Unmarshal(data []byte) error {
	if len(data) < HeaderSize {
		return ErrInvalidPacket
	}

	p.Sequence = binary.LittleEndian.Uint16(data[0:2])
	p.Ack = binary.LittleEndian.Uint16(data[2:4])
	p.AckBits = binary.LittleEndian.Uint32(data[4:8])
	p.Mode = DeliveryMode(data[8])
	dataSize := int(binary.LittleEndian.Uint16(data[9:11]))

	if len(data) < HeaderSize+dataSize {
		return ErrInvalidPacket
	}

	p.Data = make([]byte, dataSize)
	copy(p.Data, data[HeaderSize:HeaderSize+dataSize])

	return nil
}

// IsReliable returns true if this packet requires acknowledgment
func (p *Packet) IsReliable() bool {
	return p.Mode == Reliable || p.Mode == ReliableOrdered
}

// IsOrdered returns true if this packet must be delivered in order
func (p *Packet) IsOrdered() bool {
	return p.Mode == UnreliableOrdered || p.Mode == ReliableOrdered
}
