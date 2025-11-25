package prober

import (
	"fmt"
	"sync"
	"time"

	"github.com/bio-routing/matroschka-prober/pkg/target"
)

type transitProbe struct {
	target    *target.Target
	timestamp int64
}

type transitProbes struct {
	m map[uint64]transitProbe // index is the sequence number
	l sync.RWMutex
}

func (t *transitProbes) add(target *target.Target, p *target.Probe) {
	t.l.Lock()
	defer t.l.Unlock()
	t.m[p.SequenceNumber] = transitProbe{
		target:    target,
		timestamp: p.TimeStampUnixNano,
	}
}

func (t *transitProbes) remove(seq uint64) (*target.Target, error) {
	t.l.Lock()

	if _, ok := t.m[seq]; !ok {
		t.l.Unlock()
		return nil, fmt.Errorf("sequence number %d not found", seq)
	}

	tp := t.m[seq]
	delete(t.m, seq)
	t.l.Unlock()

	return tp.target, nil
}

func (t *transitProbes) getLt(lt time.Time) []uint64 {
	ret := make([]uint64, 0)
	t.l.RLock()
	defer t.l.RUnlock()

	for seq, tp := range t.m {
		if tp.timestamp < lt.UnixNano() {
			ret = append(ret, seq)
		}
	}

	return ret
}

func newTransitProbes() *transitProbes {
	return &transitProbes{
		m: make(map[uint64]transitProbe),
	}
}
