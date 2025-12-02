package prober

import (
	"time"

	log "github.com/sirupsen/logrus"
)

func (p *Prober) rttTimeoutChecker() {
	t := time.NewTicker(p.measurementLength)

	for {
		select {
		case <-p.stop:
			return
		case <-t.C:
			now := time.Now()
			maxTS := now.Add(-3 * p.measurementLength)
			for _, seq := range p.transitProbes.getLt(maxTS) {
				_, err := p.transitProbes.remove(seq)
				if err != nil {
					log.Infof("Probe %d timeouted: Unable to remove: %v", seq, err)
				}
			}
		}
	}
}
