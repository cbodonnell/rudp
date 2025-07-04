package rudp

import (
	"time"
)

const (
	RetransmissionTimeout = 100 * time.Millisecond
	MaxRetransmissions    = 5
)

// processOutbound handles sending queued packets
func (c *Connection) processOutbound() {
	for {
		select {
		case packet := <-c.outbound:
			c.sendPacket(packet)
		case <-c.done:
			return
		}
	}
}

// processRetransmissions handles reliable packet retransmission
func (c *Connection) processRetransmissions() {
	ticker := time.NewTicker(RetransmissionTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.checkRetransmissions()
		case <-c.done:
			return
		}
	}
}

// sendPacket transmits a packet over the wire
func (c *Connection) sendPacket(packet *Packet) {
	c.mu.Lock()
	packet.LastSent = time.Now()
	packet.Attempts++
	c.lastSent = packet.LastSent
	c.mu.Unlock()

	data := packet.Marshal()
	c.conn.WriteToUDP(data, c.addr)
}

// checkRetransmissions resends reliable packets that haven't been acknowledged
func (c *Connection) checkRetransmissions() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for seq, packet := range c.pendingAcks {
		if now.Sub(packet.LastSent) > RetransmissionTimeout {
			if packet.Attempts >= MaxRetransmissions {
				delete(c.pendingAcks, seq)
				continue
			}

			select {
			case c.outbound <- packet:
			default:
				// Buffer full, skip this round
			}
		}
	}
}

// HandleIncomingPacket processes received packets
func (c *Connection) HandleIncomingPacket(data []byte) error {
	packet := &Packet{}
	if err := packet.Unmarshal(data); err != nil {
		return err
	}

	c.mu.Lock()
	c.lastReceived = time.Now()

	// Process acknowledgments
	c.processAcknowledgments(packet.Ack, packet.AckBits)

	// Update remote sequence tracking
	if sequenceGreater(packet.Sequence, c.remoteSequence) {
		c.updateAckBits(packet.Sequence)
		c.remoteSequence = packet.Sequence
	}
	c.mu.Unlock()

	// Handle packet based on delivery mode
	c.handlePacketDelivery(packet)

	return nil
}

// processAcknowledgments removes acknowledged packets from pending list
func (c *Connection) processAcknowledgments(ack uint16, ackBits uint32) {
	// Acknowledge the explicit ack
	delete(c.pendingAcks, ack)

	// Process ack bits for previous packets
	for i := uint32(0); i < 32; i++ {
		if (ackBits & (1 << i)) != 0 {
			seq := ack - uint16(i+1)
			delete(c.pendingAcks, seq)
		}
	}
}

// updateAckBits updates the acknowledgment bitfield
func (c *Connection) updateAckBits(newSeq uint16) {
	diff := newSeq - c.remoteSequence
	if diff > 32 {
		c.ackBits = 0
	} else {
		c.ackBits = (c.ackBits << diff) | 1
	}
}

// handlePacketDelivery processes packet based on delivery guarantees
func (c *Connection) handlePacketDelivery(packet *Packet) {
	if packet.IsOrdered() {
		c.handleOrderedDelivery(packet)
	} else {
		c.deliverPacket(packet)
	}
}

// handleOrderedDelivery ensures packets are delivered in sequence
func (c *Connection) handleOrderedDelivery(packet *Packet) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.orderedBuffer[packet.Sequence] = packet

	// Deliver consecutive packets
	for {
		if p, exists := c.orderedBuffer[c.remoteSequence+1]; exists {
			delete(c.orderedBuffer, c.remoteSequence+1)
			c.remoteSequence++
			go c.deliverPacket(p)
		} else {
			break
		}
	}
}

// deliverPacket sends packet to the application
func (c *Connection) deliverPacket(packet *Packet) {
	select {
	case c.inbound <- packet:
	case <-c.done:
	}
}

// sequenceGreater compares sequence numbers handling wraparound
func sequenceGreater(s1, s2 uint16) bool {
	return ((s1 > s2) && (s1-s2 <= 32768)) || ((s1 < s2) && (s2-s1 > 32768))
}
