package target

import (
	"net"
	"slices"
	"sync/atomic"

	"github.com/bio-routing/matroschka-prober/pkg/config"
)

type Label struct {
	Key   string
	Value string
}

type TargetID struct {
	Path      string
	IPVersion uint8
	TOS       TOS
}

// Target keeps the state of a target instance. There is one instance per probed path.
type Target struct {
	cfg         TargetConfig
	localAddr   net.IP
	latePackets uint64
}

func NewTarget(cfg TargetConfig, localAddr net.IP) (*Target, error) {
	return &Target{
		cfg:       cfg,
		localAddr: localAddr,
	}, nil
}

func (t *Target) Config() TargetConfig {
	return t.cfg
}

// TargetConfig is the configuration of a prober
type TargetConfig struct {
	Name                string
	TOS                 TOS
	Hops                []config.Hop
	SrcAddrs            []net.IP
	StaticLabels        []Label
	MeasurementLengthMS uint64
	TimeoutMS           uint64
}

func (tc *TargetConfig) GetID() TargetID {
	return TargetID{
		Path: tc.Name,
		TOS:  tc.TOS,
	}
}

func (tc *TargetConfig) GetSrcAddr(s uint64) net.IP {
	return tc.SrcAddrs[s%uint64(len(tc.SrcAddrs))]
}

func (c *TargetConfig) Equal(b *TargetConfig) bool {
	if c == nil && b == nil {
		return true
	}
	if c == nil || b == nil {
		return false
	}

	return c.MeasurementLengthMS == b.MeasurementLengthMS &&
		c.TimeoutMS == b.TimeoutMS &&
		config.HopListsEqual(c.Hops, b.Hops) &&
		slices.Equal(c.StaticLabels, b.StaticLabels)
}

func (t *Target) SetLocalAddr(addr net.IP) {
	t.localAddr = addr
}

func (t *Target) LatePacket() {
	atomic.AddUint64(&t.latePackets, 1)
}

func (t *Target) GetLatePackets() uint64 {
	return atomic.LoadUint64(&t.latePackets)
}

func (t *Target) Labels() []string {
	keys := make([]string, len(t.cfg.StaticLabels)+2)
	for i, l := range t.cfg.StaticLabels {
		keys[i] = l.Key
	}

	keys[len(keys)-2] = "tos"
	keys[len(keys)-1] = "path"
	return keys
}

func (t *Target) LabelValues() []string {
	values := make([]string, len(t.cfg.StaticLabels)+2)
	for i, l := range t.cfg.StaticLabels {
		values[i] = l.Value
	}

	values[len(values)-2] = t.cfg.TOS.Name
	values[len(values)-1] = t.cfg.Name
	return values
}

func Targets(p config.Path, c *config.Config) []TargetConfig {
	ret := make([]TargetConfig, 0)
	for _, class := range c.Classes {
		hops, err := c.PathToProberHops(p)
		if err != nil {
			panic(err)
		}

		ret = append(ret, TargetConfig{
			Name: p.Name,
			TOS: TOS{
				Name:  class.Name,
				Value: class.TOS,
			},
			Hops:                hops,
			SrcAddrs:            config.GenerateAddrs(c.SrcRange),
			StaticLabels:        convertLabels(p.Labels),
			MeasurementLengthMS: *p.MeasurementLengthMS,
			TimeoutMS:           *p.TimeoutMS,
		})
	}

	return ret
}

func convertLabels(kv map[string]string) []Label {
	labels := make([]Label, 0, len(kv))
	for k, v := range kv {
		labels = append(labels, Label{
			Key:   k,
			Value: v,
		})
	}
	return labels
}
