package probermanager

import (
	"fmt"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/bio-routing/matroschka-prober/pkg/config"
	"github.com/bio-routing/matroschka-prober/pkg/prober"
	"github.com/bio-routing/matroschka-prober/pkg/target"
	"github.com/prometheus/client_golang/prometheus"
)

type ProberManager struct {
	probers     map[uint64][]*prober.Prober
	probersMu   sync.RWMutex
	basePort    uint16
	proberAddr4 net.IP
	proberAddr6 net.IP
	timeout     time.Duration
	rmem        int
}

func New(basePort uint16, proberAddr4 net.IP, proberAddr6 net.IP, timeout time.Duration, rmem int) *ProberManager {
	return &ProberManager{
		probers:     make(map[uint64][]*prober.Prober),
		basePort:    basePort,
		proberAddr4: proberAddr4,
		proberAddr6: proberAddr6,
		timeout:     timeout,
		rmem:        rmem,
	}
}

func (pm *ProberManager) GetProbers(pps uint64) ([]*prober.Prober, error) {
	pm.probersMu.Lock()
	defer pm.probersMu.Unlock()

	if p, ok := pm.probers[pps]; ok {
		return p, nil
	}

	pm.probers[pps] = make([]*prober.Prober, 0, runtime.GOMAXPROCS(0))
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		p := prober.New(pps, pm.basePort, pm.proberAddr4, pm.proberAddr6, pm.timeout, pm.rmem)
		err := p.Start()
		if err != nil {
			return nil, fmt.Errorf("unable to start prober: %v", err)
		}

		pm.probers[pps] = append(pm.probers[pps], p)
	}

	return pm.probers[pps], nil
}

func (pm *ProberManager) Configure(cfg *config.Config) error {
	pathsByPPSRate := make(map[uint64][]config.Path)
	for _, path := range cfg.Paths {
		pps := *path.PPS
		pathsByPPSRate[pps] = append(pathsByPPSRate[pps], path)
	}

	pm.terminateUnneededProbers(pathsByPPSRate)

	for pps, paths := range pathsByPPSRate {
		targetConfigs := make([]target.TargetConfig, 0)
		for _, path := range paths {
			targetConfigs = append(targetConfigs, target.Targets(path, cfg)...)
		}

		probers, err := pm.GetProbers(pps)
		if err != nil {
			return fmt.Errorf("unable to get prober for pps %d: %v", pps, err)
		}

		for i, group := range splitTargetConfigs(targetConfigs, len(probers)) {
			err = probers[i].Configure(group)
			if err != nil {
				return fmt.Errorf("failed to configure probers %d/%d: %v", pps, i, err)
			}
		}
	}

	return nil
}

func (pm *ProberManager) terminateUnneededProbers(neededProbers map[uint64][]config.Path) {
	probersToStop := make([]uint64, 0)

	pm.probersMu.Lock()
	defer pm.probersMu.Unlock()

	for pps := range pm.probers {
		if _, exists := neededProbers[pps]; !exists {
			probersToStop = append(probersToStop, pps)
		}
	}

	for _, pps := range probersToStop {
		for _, p := range pm.probers[pps] {
			p.Stop()
		}

		delete(pm.probers, pps)
	}
}

func splitTargetConfigs(targets []target.TargetConfig, nGroups int) [][]target.TargetConfig {
	ret := make([][]target.TargetConfig, nGroups)
	for i := range nGroups {
		ret[i] = make([]target.TargetConfig, 0, len(targets)/nGroups)
	}

	for i, t := range targets {
		ret[i%nGroups] = append(ret[i%nGroups], t)
	}

	return ret
}

func (pm *ProberManager) GetCollectors() []prometheus.Collector {
	ret := make([]prometheus.Collector, 0)
	pm.probersMu.RLock()
	defer pm.probersMu.RUnlock()

	for _, probers := range pm.probers {
		for _, p := range probers {
			ret = append(ret, p)
		}
	}

	return ret
}
