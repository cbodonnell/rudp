## RUDP Packet - Packet structure and serialization matching packet.go
## Based on github.com/cbodonnell/rudp/packet.go
extends RefCounted
class_name RUDPPacket

# PacketType defines the type of packet (matching Go)
enum PacketType {
	DATA = 0,
	CONNECT = 1,
	CONNECT_ACK = 2,
	DISCONNECT = 3
}

# DeliveryMode defines how packets should be delivered (matching Go)
enum DeliveryMode {
	UNRELIABLE = 0,         # Fire and forget, no guarantees
	UNRELIABLE_ORDERED = 1, # Ordered delivery, but may drop old packets
	RELIABLE = 2,           # Guaranteed delivery, may arrive out of order
	RELIABLE_ORDERED = 3    # Guaranteed delivery in sequence order
}

# Constants (matching Go packet.go)
const MAX_PACKET_SIZE = 1400  # bytes
const HEADER_SIZE = 16        # Type(1) + ClientID(4) + Seq(2) + Ack(2) + AckBits(4) + Mode(1) + DataSize(2)

# Packet represents a network packet with metadata (matching Go struct)
var type: int = PacketType.DATA  # PacketType (byte)
var client_id: int = 0        # uint32 - Unique client identifier
var id: int = 0               # uint16 (not used in wire protocol)
var sequence: int = 0         # uint16
var ack: int = 0              # uint16
var ack_bits: int = 0         # uint32
var mode: int = 0             # DeliveryMode (byte)
var timestamp: int = 0        # int64 (not used in wire protocol)
var data: PackedByteArray = PackedByteArray()
var attempts: int = 0         # For retransmission tracking
var last_sent: int = 0        # Time.get_ticks_msec()

## Marshal serializes the packet for network transmission (matching Go)
func marshal() -> PackedByteArray:
	# All packets use the same format (CONNECT/CONNECT_ACK just leave Seq/Ack/etc at 0)
	var buf = PackedByteArray()
	buf.resize(HEADER_SIZE + data.size())

	# Write header (Little Endian) - matching Go binary.LittleEndian
	buf[0] = type                           # buf[0] Type
	buf.encode_u32(1, client_id)            # buf[1:5] ClientID
	buf.encode_u16(5, sequence & 0xFFFF)    # buf[5:7] Sequence
	buf.encode_u16(7, ack & 0xFFFF)         # buf[7:9] Ack
	buf.encode_u32(9, ack_bits)             # buf[9:13] AckBits
	buf[13] = mode                          # buf[13] Mode
	buf.encode_u16(14, data.size())         # buf[14:16] DataSize

	# Copy payload data - matching Go copy(buf[HeaderSize:], p.Data)
	for i in range(data.size()):
		buf[HEADER_SIZE + i] = data[i]

	return buf

## Unmarshal deserializes a packet from network data (matching Go)
func unmarshal(raw_data: PackedByteArray) -> int:
	if raw_data.size() < HEADER_SIZE:
		return ERR_INVALID_PARAMETER  # ErrInvalidPacket

	# Read header (Little Endian) - matching Go binary.LittleEndian
	type = raw_data[0]                      # data[0] Type
	client_id = raw_data.decode_u32(1)      # data[1:5] ClientID
	sequence = raw_data.decode_u16(5)       # data[5:7] Sequence
	ack = raw_data.decode_u16(7)            # data[7:9] Ack
	ack_bits = raw_data.decode_u32(9)       # data[9:13] AckBits
	mode = raw_data[13]                     # data[13] Mode
	var data_size = raw_data.decode_u16(14) # data[14:16] DataSize

	if raw_data.size() < HEADER_SIZE + data_size:
		return ERR_INVALID_PARAMETER  # ErrInvalidPacket

	# Copy payload data - matching Go copy(p.Data, data[HeaderSize:])
	data = raw_data.slice(HEADER_SIZE, HEADER_SIZE + data_size)

	return OK

## IsReliable returns true if this packet requires acknowledgment (matching Go)
func is_reliable() -> bool:
	return mode == DeliveryMode.RELIABLE or mode == DeliveryMode.RELIABLE_ORDERED

## IsOrdered returns true if this packet must be delivered in order (matching Go)
func is_ordered() -> bool:
	return mode == DeliveryMode.UNRELIABLE_ORDERED or mode == DeliveryMode.RELIABLE_ORDERED
