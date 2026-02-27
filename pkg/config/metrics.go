package config

import (
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
)

// ReloadFailed tracks whether the last config reload failed (0 or 1).
type ReloadFailed struct {
	failed atomic.Bool
}

func NewReloadFailed() *ReloadFailed {
	return &ReloadFailed{}
}

func (m *ReloadFailed) SetFailed() {
	m.failed.Store(true)
}

func (m *ReloadFailed) SetOK() {
	m.failed.Store(false)
}

func (m *ReloadFailed) Describe(ch chan<- *prometheus.Desc) {}

func (m *ReloadFailed) Collect(ch chan<- prometheus.Metric) {
	desc := prometheus.NewDesc("matroschka_config_reload_failed", "1 if the last config reload failed, 0 otherwise.", nil, nil)
	var val float64
	if m.failed.Load() {
		val = 1
	}
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, val)
}
