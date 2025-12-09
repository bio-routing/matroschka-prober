package measurement

import (
	"slices"
	"sync"
	"time"

	"github.com/bio-routing/matroschka-prober/pkg/target"
	log "github.com/sirupsen/logrus"
)

// Measurement represents a measurement
type Measurement struct {
	Sent     uint64
	Received uint64
	RTTSum   uint64
	RTTMin   uint64
	RTTMax   uint64
	RTTs     []uint64
}

func (m *Measurement) copy() *Measurement {
	return &Measurement{
		Sent:     m.Sent,
		Received: m.Received,
		RTTSum:   m.RTTSum,
		RTTMin:   m.RTTMin,
		RTTMax:   m.RTTMax,
		RTTs:     slices.Clone(m.RTTs),
	}
}

// MeasurementsDB manages measurements
type MeasurementsDB struct {
	m map[int64]map[*target.Target]*Measurement
	l sync.RWMutex
}

// NewDB creates a new measurements database
func NewDB() *MeasurementsDB {
	return &MeasurementsDB{
		m: make(map[int64]map[*target.Target]*Measurement),
	}
}

// AddSent adds a sent probe to the db
func (m *MeasurementsDB) AddSent(t *target.Target, ts int64) {
	m.l.Lock()

	if m.m[ts] == nil {
		m.m[ts] = make(map[*target.Target]*Measurement)
	}

	if m.m[ts][t] == nil {
		m.m[ts][t] = &Measurement{}
	}

	m.m[ts][t].Sent++
	m.l.Unlock() // This is not defered for performance reason
}

// AddRecv adds a received probe to the db
func (m *MeasurementsDB) AddRecv(sentTsNS int64, rtt uint64, t *target.Target) {
	m.l.RLock()

	allignedTs := sentTsNS - sentTsNS%int64(t.Config().MeasurementLengthMS*uint64(time.Millisecond))
	if _, ok := m.m[allignedTs]; !ok {
		log.Debugf("Received probe at %d sent at %d with rtt %d after bucket %d was removed. Now=%d", sentTsNS+int64(rtt), sentTsNS, allignedTs, rtt, time.Now().UnixNano())
		m.l.RUnlock() // This is not defered for performance reason
		return
	}

	if m.m[allignedTs] == nil {
		m.l.RUnlock()
		return
	}

	if m.m[allignedTs][t] == nil {
		m.l.RUnlock()
		return
	}

	me := m.m[allignedTs][t]
	me.Received++
	me.RTTs = append(me.RTTs, rtt)
	me.RTTSum += rtt

	if rtt < me.RTTMin || me.RTTMin == 0 {
		me.RTTMin = rtt
	}

	if rtt > me.RTTMax {
		me.RTTMax = rtt
	}

	m.l.RUnlock() // This is not defered for performance reason
}

// RemoveOlder removes all probes from the db that are older than ts
func (m *MeasurementsDB) RemoveOlder(ts int64) {
	m.l.Lock()
	defer m.l.Unlock()

	for t := range m.m {
		if t < ts {
			delete(m.m, t)
		}
	}
}

// Get get's the measurement at ts
func (m *MeasurementsDB) Get(ts int64, t *target.Target) *Measurement {
	m.l.RLock()
	defer m.l.RUnlock()

	if _, ok := m.m[ts]; !ok {
		return nil
	}

	return m.m[ts][t].copy()
}
