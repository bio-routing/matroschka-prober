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

		_, err := p.udpConn.Read(recvBuffer)
		now := time.Now().UnixNano()
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

		rtt := now - pkt.TimeStampUnixNano
		if p.timedOut(rtt, target) {
			// Probe arrived late. rttTimoutChecker() will clean up after it. So we ignore it from here on
			target.LatePacket()
			continue
		}

		p.measurements.AddRecv(pkt.TimeStampUnixNano, uint64(rtt), target)
	}
}

func (p *Prober) timedOut(s int64, target *target.Target) bool {
	return s > int64(msToNS(target.Config().TimeoutMS))
}

func msToNS(s uint64) uint64 {
	return s * 1000000
}
