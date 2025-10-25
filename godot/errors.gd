## RUDP Errors - Error constants matching errors.go
## Based on github.com/cbodonnell/rudp/errors.go
extends RefCounted
class_name RUDPErrors

# Error constants (matching Go errors.go)
const ERR_INVALID_PACKET = "invalid packet format"
const ERR_PACKET_TOO_LARGE = "packet exceeds maximum size"
const ERR_CONNECTION_CLOSED = "connection is closed"
const ERR_TIMEOUT = "operation timed out"
const ERR_BUFFER_FULL = "send buffer is full"
