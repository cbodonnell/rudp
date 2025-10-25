## RUDP Reliability - Reliability and retransmission logic matching reliability.go
## Based on github.com/cbodonnell/rudp/reliability.go
extends RefCounted
class_name RUDPReliability

# Constants (matching Go reliability.go)
const RETRANSMISSION_TIMEOUT = 100  # milliseconds
const MAX_RETRANSMISSIONS = 5

## sequenceGreater compares sequence numbers handling wraparound (matching Go)
static func sequence_greater(s1: int, s2: int) -> bool:
	s1 = s1 & 0xFFFF
	s2 = s2 & 0xFFFF
	return ((s1 > s2) and (s1 - s2 <= 32768)) or ((s1 < s2) and (s2 - s1 > 32768))
