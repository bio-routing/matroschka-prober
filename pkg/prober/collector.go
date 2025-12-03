package prober

import (
	"time"

	"github.com/bio-routing/matroschka-prober/pkg/measurement"
	"github.com/bio-routing/matroschka-prober/pkg/target"
	"github.com/prometheus/client_golang/prometheus"

	log "github.com/sirupsen/logrus"
)

const (
	metricPrefix = "matroschka_"
)

// Describe is required by prometheus interface
func (p *Prober) Describe(ch chan<- *prometheus.Desc) {
}

// Collect collects data from the collector and send it to prometheus
func (p *Prober) Collect(ch chan<- prometheus.Metric) {
	p.targetsMu.RLock()
	defer p.targetsMu.RUnlock()

	for _, t := range p.targets {
		ts := p.lastFinishedMeasurement(t)
		m := p.measurements.Get(ts, t)
		if m == nil {
			log.Debugf("Requested timestamp %d not found", ts)

			return
		}

		p.collectSent(ch, m, t)
		p.collectReceived(ch, m, t)
		p.collectRTTMin(ch, m, t)
		p.collectRTTMax(ch, m, t)
		p.collectRTTAvg(ch, m, t)
		p.collectLatePackets(ch, t)
	}

}

func (p *Prober) collectSent(ch chan<- prometheus.Metric, m *measurement.Measurement, t *target.Target) {
	desc := prometheus.NewDesc(metricPrefix+"packets_sent", "Sent packets", t.Labels(), nil)
	ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, float64(m.Sent), t.LabelValues()...)
}

func (p *Prober) collectReceived(ch chan<- prometheus.Metric, m *measurement.Measurement, t *target.Target) {
	desc := prometheus.NewDesc(metricPrefix+"packets_received", "Received packets", t.Labels(), nil)
	ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, float64(m.Received), t.LabelValues()...)
}

func (p *Prober) collectRTTMin(ch chan<- prometheus.Metric, m *measurement.Measurement, t *target.Target) {
	desc := prometheus.NewDesc(metricPrefix+"rtt_min", "RTT Min [nanoseconds]", t.Labels(), nil)
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(m.RTTMin), t.LabelValues()...)
}

func (p *Prober) collectRTTMax(ch chan<- prometheus.Metric, m *measurement.Measurement, t *target.Target) {
	desc := prometheus.NewDesc(metricPrefix+"rtt_max", "RTT Max [nanoseconds]", t.Labels(), nil)
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(m.RTTMax), t.LabelValues()...)
}

func (p *Prober) collectRTTAvg(ch chan<- prometheus.Metric, m *measurement.Measurement, t *target.Target) {
	desc := prometheus.NewDesc(metricPrefix+"rtt_avg", "RTT Average [nanoseconds]", t.Labels(), nil)
	v := float64(0)
	if m.Received != 0 {
		v = float64(m.RTTSum / m.Received)
	}
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, t.LabelValues()...)
}

func (p *Prober) collectLatePackets(ch chan<- prometheus.Metric, t *target.Target) {
	desc := prometheus.NewDesc(metricPrefix+"late_packets_total", "Timedout but received packets", t.Labels(), nil)
	n := t.GetLatePackets()
	ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, float64(n), t.LabelValues()...)
}

func (p *Prober) lastFinishedMeasurement(t *target.Target) int64 {
	measurementLengthNS := int64(t.Config().MeasurementLengthMS) * int64(time.Millisecond)
	timeoutNS := int64(t.Config().TimeoutMS) * int64(time.Millisecond)
	nowNS := p.clock.Now().UnixNano()
	ts := nowNS - timeoutNS - measurementLengthNS
	return ts - ts%measurementLengthNS
}
