package prober

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/bio-routing/matroschka-prober/pkg/target"
	"golang.org/x/sys/unix"

	log "github.com/sirupsen/logrus"
)

func (p *Prober) sender() {
	defer p.rawConn4.Close()
	defer p.rawConn6.Close()

	seq := uint64(0)
	pr := target.Probe{}

	ticker := time.NewTicker(time.Second / time.Duration(p.pps))

	for {
		select {
		case <-p.stop:
			return
		case <-ticker.C:
		}

		p.targetsMu.RLock()
		for _, target := range p.targets {
			tCfg := target.Config()

			srcAddr := tCfg.GetSrcAddr(seq)
			dstAddr := tCfg.Hops[0].GetAddr(seq)

			pr.SequenceNumber = seq
			pr.TimeStampUnixNano = time.Now().UnixNano()
			pkt, err := target.CraftPacket(pr, p.udpPort)
			if err != nil {
				log.Errorf("Unable to craft packet: %v", err)
				continue
			}

			p.transitProbes.add(target, &pr)

			tsAligned := pr.TimeStampUnixNano - (pr.TimeStampUnixNano % (int64(tCfg.MeasurementLengthMS) * int64(time.Millisecond)))
			p.measurements.AddSent(target, tsAligned)

			err = p.sendPacket(pkt, srcAddr, dstAddr, tCfg.TOS.Value)
			if err != nil {
				log.Errorf("Unable to send packet: %v", err)
				_, err = p.transitProbes.remove(pr.SequenceNumber)
				if err != nil {
					log.Errorf("unable to remove transit probe %d: %v", pr.SequenceNumber, err)
				}

				continue
			}

			atomic.AddUint64(&p.probesSent, 1)
			seq++
		}
		p.targetsMu.RUnlock()
	}
}

func (p *Prober) sendPacket(payload []byte, src net.IP, dst net.IP, tos uint8) error {
	options := writeOptions{
		src:      src,
		dst:      dst,
		tos:      int64(tos),
		ttl:      ttl,
		protocol: unix.IPPROTO_GRE,
	}

	if dst.To4() != nil {
		if err := p.rawConn4.WriteTo(payload, options); err != nil {
			return fmt.Errorf("unable to send ipv4 packet: %v", err)
		}
		return nil
	}

	if err := p.rawConn6.WriteTo(payload, options); err != nil {
		return fmt.Errorf("unable to send ipv6 packet: %v", err)
	}

	return nil
}
