## RUDP Connection - Connection management matching connection.go
## Based on github.com/cbodonnell/rudp/connection.go and reliability.go
extends RefCounted
class_name RUDPConnection

# Constants (matching Go connection.go)
const INACTIVITY_TIMEOUT = 5.0  # seconds

# Connection represents a reliable UDP connection to a peer (matching Go struct)
var _addr: String = ""  # Remote IP
var _port: int = 0      # Remote port
var _conn: PacketPeerUDP = null

# Identity (matching Go)
var _client_id: int = 0  # uint32

# Sequence tracking (matching Go)
var _local_sequence: int = 0   # uint16
var _remote_sequence: int = 0  # uint16
var _ack_bits: int = 0         # uint32

# Reliability (matching Go)
var _pending_acks: Dictionary = {}    # map[uint16]*Packet
var _ordered_buffer: Dictionary = {}  # map[uint16]*Packet

# State (matching Go)
var _last_received: float = 0.0  # Time.get_ticks_msec() / 1000.0
var _last_sent: float = 0.0
var _closed: bool = false

# Channels (emulated as queues since GDScript doesn't have goroutines)
var _inbound: Array = []   # chan *Packet (buffered 256)
var _outbound: Array = []  # chan *Packet (buffered 256)

const CHANNEL_BUFFER_SIZE = 256

## NewConnection creates a new connection to the specified address (matching Go)
func _init(conn: PacketPeerUDP, address: String, port: int, client_id: int):
	_conn = conn
	_addr = address
	_port = port
	_client_id = client_id
	_last_received = Time.get_ticks_msec() / 1000.0

## Send queues a packet for transmission (matching Go connection.go:65)
func send(data: PackedByteArray, mode: int) -> int:
	if _closed:
		return ERR_UNCONFIGURED  # ErrConnectionClosed

	if data.size() > RUDPPacket.MAX_PACKET_SIZE - RUDPPacket.HEADER_SIZE:
		return ERR_INVALID_PARAMETER  # ErrPacketTooLarge

	var packet = RUDPPacket.new()
	packet.type = RUDPPacket.PacketType.DATA
	packet.client_id = _client_id
	packet.sequence = _local_sequence
	packet.ack = _remote_sequence
	packet.ack_bits = _ack_bits
	packet.mode = mode
	packet.data = data
	packet.timestamp = Time.get_ticks_msec()

	_local_sequence = (_local_sequence + 1) & 0xFFFF

	if packet.is_reliable():
		_pending_acks[packet.sequence] = packet

	# Add to outbound queue (matching Go: c.outbound <- packet)
	if _outbound.size() < CHANNEL_BUFFER_SIZE:
		_outbound.append(packet)
		return OK
	else:
		return ERR_BUSY  # ErrBufferFull

## Receive returns the next available packet (matching Go connection.go:103)
func receive() -> RUDPPacket:
	if _inbound.size() > 0:
		return _inbound.pop_front()
	if _closed:
		return null  # ErrConnectionClosed
	return null  # No packet available

## Close closes the connection (matching Go connection.go:113)
func close() -> int:
	if _closed:
		return OK

	_closed = true
	return OK

## IsConnected returns true if the connection is active (matching Go connection.go:127)
func is_connection_active() -> bool:
	var now = Time.get_ticks_msec() / 1000.0
	return not _closed and (now - _last_received) < INACTIVITY_TIMEOUT

## RemoteAddr returns the remote address string (matching Go connection.go:134)
func remote_addr() -> String:
	return "%s:%d" % [_addr, _port]

## ClientID returns the client ID of the connection (matching Go connection.go:139)
func get_client_id() -> int:
	return _client_id

## UpdateAddr updates the remote address (for handling reconnections) (matching Go connection.go:144)
func update_addr(address: String, port: int) -> void:
	_addr = address
	_port = port

## process_outbound handles sending queued packets (matching Go reliability.go:13)
func process_outbound() -> void:
	while _outbound.size() > 0:
		var packet = _outbound.pop_front()
		send_packet(packet)

## send_packet transmits a packet over the wire (matching Go reliability.go:40)
func send_packet(packet: RUDPPacket) -> void:
	packet.last_sent = Time.get_ticks_msec()
	packet.attempts += 1
	_last_sent = packet.last_sent / 1000.0

	var data = packet.marshal()
	_conn.put_packet(data)

## check_retransmissions resends reliable packets that haven't been acknowledged (matching Go reliability.go:52)
func check_retransmissions() -> void:
	var now = Time.get_ticks_msec()
	var to_remove = []

	for seq in _pending_acks.keys():
		var packet: RUDPPacket = _pending_acks[seq]
		if now - packet.last_sent > RUDPReliability.RETRANSMISSION_TIMEOUT:
			if packet.attempts >= RUDPReliability.MAX_RETRANSMISSIONS:
				to_remove.append(seq)
				continue

			# Resend packet (matching Go: c.outbound <- packet)
			if _outbound.size() < CHANNEL_BUFFER_SIZE:
				_outbound.append(packet)

	for seq in to_remove:
		_pending_acks.erase(seq)

## handle_incoming_packet processes received packets (matching Go reliability.go:74)
func handle_incoming_packet(packet: RUDPPacket) -> int:
	_last_received = Time.get_ticks_msec() / 1000.0

	# Process acknowledgments (matching Go reliability.go:84)
	process_acknowledgments(packet.ack, packet.ack_bits)

	# Update remote sequence tracking (matching Go reliability.go:87)
	if RUDPReliability.sequence_greater(packet.sequence, _remote_sequence):
		update_ack_bits(packet.sequence)
		_remote_sequence = packet.sequence

	# Handle packet based on delivery mode (matching Go reliability.go:94)
	handle_packet_delivery(packet)

	return OK

## process_acknowledgments removes acknowledged packets from pending list (matching Go reliability.go:95)
func process_acknowledgments(ack: int, ack_bits_received: int) -> void:
	# Acknowledge the explicit ack
	_pending_acks.erase(ack)

	# Process ack bits for previous packets
	for i in range(32):
		if (ack_bits_received & (1 << i)) != 0:
			var seq = (ack - (i + 1)) & 0xFFFF
			_pending_acks.erase(seq)

## update_ack_bits updates the acknowledgment bitfield (matching Go reliability.go:109)
func update_ack_bits(new_seq: int) -> void:
	var diff = new_seq - _remote_sequence
	if diff > 32:
		_ack_bits = 0
	else:
		_ack_bits = (_ack_bits << diff) | 1

## handle_packet_delivery processes packet based on delivery guarantees (matching Go reliability.go:119)
func handle_packet_delivery(packet: RUDPPacket) -> void:
	if packet.is_ordered():
		handle_ordered_delivery(packet)
	else:
		deliver_packet(packet)

## handle_ordered_delivery ensures packets are delivered in sequence (matching Go reliability.go:128)
func handle_ordered_delivery(packet: RUDPPacket) -> void:
	_ordered_buffer[packet.sequence] = packet

	# Deliver consecutive packets (matching Go reliability.go:134)
	while true:
		var next_seq = (_remote_sequence + 1) & 0xFFFF
		if _ordered_buffer.has(next_seq):
			var p = _ordered_buffer[next_seq]
			_ordered_buffer.erase(next_seq)
			_remote_sequence = next_seq
			deliver_packet(p)
		else:
			break

## deliver_packet sends packet to the application (matching Go reliability.go:147)
func deliver_packet(packet: RUDPPacket) -> void:
	if _inbound.size() < CHANNEL_BUFFER_SIZE:
		_inbound.append(packet)
