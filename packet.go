package rudp

import (
	"encoding/binary"
	"time"
)

// PacketType defines the type of packet
type PacketType byte

const (
	DATA PacketType = iota
	CONNECT
	CONNECT_ACK
	DISCONNECT
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
	Type      PacketType
	ClientID  uint32 // Unique client identifier (in all packets)
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

const HeaderSize = 16 // Type(1) + ClientID(4) + Seq(2) + Ack(2) + AckBits(4) + Mode(1) + DataSize(2)

// Marshal serializes the packet for network transmission
func (p *Packet) Marshal() []byte {
	// All packets use the same format (CONNECT/CONNECT_ACK just leave Seq/Ack/etc at 0)
	buf := make([]byte, HeaderSize+len(p.Data))
	buf[0] = byte(p.Type)
	binary.LittleEndian.PutUint32(buf[1:5], p.ClientID)
	binary.LittleEndian.PutUint16(buf[5:7], p.Sequence)
	binary.LittleEndian.PutUint16(buf[7:9], p.Ack)
	binary.LittleEndian.PutUint32(buf[9:13], p.AckBits)
	buf[13] = byte(p.Mode)
	binary.LittleEndian.PutUint16(buf[14:16], uint16(len(p.Data)))
	copy(buf[HeaderSize:], p.Data)
	return buf
}

// Unmarshal deserializes a packet from network data
func (p *Packet) Unmarshal(data []byte) error {
	if len(data) < HeaderSize {
		return ErrInvalidPacket
	}

	p.Type = PacketType(data[0])
	p.ClientID = binary.LittleEndian.Uint32(data[1:5])
	p.Sequence = binary.LittleEndian.Uint16(data[5:7])
	p.Ack = binary.LittleEndian.Uint16(data[7:9])
	p.AckBits = binary.LittleEndian.Uint32(data[9:13])
	p.Mode = DeliveryMode(data[13])
	dataSize := int(binary.LittleEndian.Uint16(data[14:16]))

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
