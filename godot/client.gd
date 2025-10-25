## RUDP Client - UDP client implementation matching client.go
## Based on github.com/cbodonnell/rudp/client.go
extends Node
class_name RUDPClient

# Client represents a UDP client connection (matching Go struct)
var _conn: PacketPeerUDP = null
var _connection: RUDPConnection = null
var _client_id: int = 0      # uint32
var _connected: bool = false

# Events (matching Go callbacks)
var on_message: Callable  # func(packet: RUDPPacket)
var on_disconnect: Callable  # func()

# State
var _done: bool = false

## NewClient creates a new UDP client (matching Go client.go:26)
func _init():
	_client_id = generate_client_id()

## generateClientID creates a random 32-bit client identifier (matching Go client.go:34)
static func generate_client_id() -> int:
	return randi() & 0xFFFFFFFF  # Generate random 32-bit unsigned integer

## Connect establishes a connection to the server (matching Go client.go:41)
func connect_to_server(addr: String) -> int:
	# Parse address (expecting "host:port")
	var parts = addr.split(":")
	if parts.size() != 2:
		return ERR_INVALID_PARAMETER

	var host = parts[0]
	var port = int(parts[1])

	# Create UDP socket (matching Go net.ListenUDP)
	_conn = PacketPeerUDP.new()
	var err = _conn.bind(0)  # Bind to any available port
	if err != OK:
		return err

	# Set destination
	_conn.set_dest_address(host, port)

	# Create connection (matching Go NewConnection)
	_connection = RUDPConnection.new(_conn, host, port, _client_id)

	# Perform handshake BEFORE starting background processing (matching Go client.go:54)
	err = perform_handshake()
	if err != OK:
		disconnect_from_server()
		return err

	return OK

## performHandshake sends CONNECT and waits for CONNECT_ACK (matching Go client.go:68)
func perform_handshake() -> Error:
	var connect_packet = RUDPPacket.new()
	connect_packet.type = RUDPPacket.PacketType.CONNECT
	connect_packet.client_id = _client_id
	connect_packet.data = PackedByteArray()

	# Send CONNECT packet (matching Go client.go:75)
	var data = connect_packet.marshal()
	_conn.put_packet(data)

	# Wait for CONNECT_ACK with timeout (no other goroutines reading yet)
	var deadline = Time.get_ticks_msec() + 5000  # 5 second timeout
	var buffer: PackedByteArray

	while Time.get_ticks_msec() < deadline:
		if _conn.get_available_packet_count() > 0:
			buffer = _conn.get_packet()

			var packet = RUDPPacket.new()
			if packet.unmarshal(buffer) != OK:
				print("Failed to unmarshal packet during handshake")
				continue

			if packet.type == RUDPPacket.PacketType.CONNECT_ACK and packet.client_id == _client_id:
				_connected = true
				print("Connected to server with clientID: %d" % _client_id)
				return OK

	return ERR_TIMEOUT  # Handshake timeout: no CONNECT_ACK received

## handle_packets reads incoming UDP packets (matching Go client.go:112)
## This would be called in _process() since GDScript doesn't use goroutines
func handle_packets() -> void:
	while _conn.get_available_packet_count() > 0:
		var buffer = _conn.get_packet()

		# Parse packet to check type (matching Go client.go:130)
		var packet = RUDPPacket.new()
		if packet.unmarshal(buffer) != OK:
			continue

		# Ignore handshake packets (already handled during Connect) (matching Go client.go:136)
		if packet.type == RUDPPacket.PacketType.CONNECT or packet.type == RUDPPacket.PacketType.CONNECT_ACK:
			continue

		_connection.handle_incoming_packet(packet)

## handle_connection processes packets from the server (matching Go client.go:149)
## This would be called in _process() since GDScript doesn't use goroutines
func handle_connection() -> void:
	var packet = _connection.receive()
	if packet != null:
		if on_message:
			on_message.call(packet)
		return

	# Check if connection closed
	if not _connection.is_connection_active():
		if on_disconnect:
			on_disconnect.call()

## Send transmits data to the server (matching Go client.go:169)
func send(data: PackedByteArray, mode: int) -> int:
	if _connection == null:
		return ERR_UNCONFIGURED  # ErrConnectionClosed
	return _connection.send(data, mode)

## IsConnected returns true if connected to server (matching Go client.go:177)
func is_connected_to_server() -> bool:
	return _connected and _connection != null and _connection.is_connection_active()

## ClientID returns the client's unique identifier (matching Go client.go:182)
func get_client_id() -> int:
	return _client_id

## Close disconnects from the server (matching Go client.go:187)
func disconnect_from_server() -> int:
	_done = true

	if _connection != null:
		_connection.close()

	if _conn != null:
		_conn.close()
		return OK

	return OK

## RemoteAddr returns the server address (matching Go client.go:201)
func remote_addr() -> String:
	if _connection != null:
		return _connection.remote_addr()
	return ""

## _process handles packet processing and connection management
## This replaces the Go goroutines
func _process(_delta: float) -> void:
	if _done or _connection == null:
		return

	# Handle incoming packets (replaces handlePackets goroutine)
	handle_packets()

	# Process outbound queue (replaces processOutbound goroutine)
	_connection.process_outbound()

	# Check for retransmissions (replaces processRetransmissions goroutine)
	_connection.check_retransmissions()

	# Handle application packets (replaces handleConnection goroutine)
	handle_connection()
