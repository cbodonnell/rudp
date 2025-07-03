package rudp

import "errors"

var (
	ErrInvalidPacket    = errors.New("invalid packet format")
	ErrPacketTooLarge   = errors.New("packet exceeds maximum size")
	ErrConnectionClosed = errors.New("connection is closed")
	ErrTimeout          = errors.New("operation timed out")
	ErrBufferFull       = errors.New("send buffer is full")
)
