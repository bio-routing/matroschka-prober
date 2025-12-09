package prober

import (
	"sync/atomic"
	"time"

	"github.com/bio-routing/matroschka-prober/pkg/target"
	log "github.com/sirupsen/logrus"
)

const (
	mtuMax = uint16(9216)
)

func (p *Prober) receiver() {
	defer p.udpConn.Close()

	recvBuffer := make([]byte, mtuMax)
	for {
		select {
		case <-p.stop:
			return
		default:
		}

		_, ts, err := p.udpConn.Read(recvBuffer)
		if ts == nil {
			now := time.Now()
			ts = &now
		}

		if err != nil {
			log.Errorf("Unable to read from UDP socket: %v", err)
			return
		}

		atomic.AddUint64(&p.probesReceived, 1)

		pkt, err := target.Unmarshal(recvBuffer)
		if err != nil {
			log.Errorf("Unable to unmarshal message: %v", err)
			return
		}

		target, err := p.transitProbes.remove(pkt.SequenceNumber)
		if err != nil {
			// Probe was count as lost, so we ignore it from here on
			continue
		}

		rtt := ts.UnixNano() - pkt.TimeStampUnixNano
		if target.TimedOut(rtt) {
			// Probe arrived late. rttTimoutChecker() will clean up after it. So we ignore it from here on
			target.LatePacket()
			continue
		}

		p.measurements.AddRecv(pkt.TimeStampUnixNano, uint64(rtt), target)
	}
}
