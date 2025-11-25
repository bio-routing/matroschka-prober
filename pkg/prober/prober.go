package prober

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/bio-routing/matroschka-prober/pkg/measurement"
	"github.com/bio-routing/matroschka-prober/pkg/target"
)

type Prober struct {
	clock             clock
	stop              chan struct{}
	rawConn4          rawSocket // Used to send GRE packets for IPv4
	rawConn6          rawSocket // Used to send GRE packets for IPv6
	proberAddr4       net.IP
	proberAddr6       net.IP
	basePort          uint16
	udpPort           uint16
	udpConn           udpSocket // Used to receive returning packets
	probesReceived    uint64
	probesSent        uint64
	targets           map[target.TargetID]*target.Target
	targetsMu         sync.RWMutex
	pps               uint64
	transitProbes     *transitProbes
	measurements      *measurement.MeasurementsDB
	measurementLength time.Duration
}

// New creates a new prober
func New(pps uint64, basePort uint16, proberAddr4 net.IP, proberAddr6 net.IP, measurementLength time.Duration) *Prober {
	pr := &Prober{
		basePort:          basePort,
		clock:             realClock{},
		stop:              make(chan struct{}),
		proberAddr4:       proberAddr4,
		proberAddr6:       proberAddr6,
		udpPort:           basePort + 1,
		targets:           make(map[target.TargetID]*target.Target),
		pps:               pps,
		transitProbes:     newTransitProbes(),
		measurements:      measurement.NewDB(),
		measurementLength: measurementLength,
	}

	return pr
}

func (p *Prober) Configure(targetConfigs []target.TargetConfig) error {
	p.targetsMu.Lock()
	defer p.targetsMu.Unlock()

	for _, tc := range targetConfigs {
		laddr, err := getLocalAddr(tc.Hops[0].GetAddr(0))
		if err != nil {
			return fmt.Errorf("unable to get local address for target %q: %v", tc.Name, err)
		}

		t, err := target.NewTarget(tc, laddr)
		if err != nil {
			return fmt.Errorf("unable to create target %q: %v", tc.Name, err)
		}
		p.targets[tc.GetID()] = t
	}

	return nil
}

func getLocalAddr(dest net.IP) (net.IP, error) {
	conn, err := net.Dial("udp", net.JoinHostPort(dest.String(), "123"))
	if err != nil {
		return nil, fmt.Errorf("dial failed: %v", err)
	}

	conn.Close()

	host, _, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		return nil, fmt.Errorf("unable to split host and port: %v", err)
	}

	return net.ParseIP(host), nil
}

// Start starts the prober
func (p *Prober) Start() error {
	err := p.init()
	if err != nil {
		return fmt.Errorf("failed to init: %v", err)
	}

	go p.rttTimeoutChecker()
	go p.sender()
	go p.receiver()
	go p.cleaner()
	return nil
}

// Stop stops the prober
func (p *Prober) Stop() {
	close(p.stop)
}

func (p *Prober) cleaner() {
	for {
		select {
		case <-p.stop:
			return
		default:
			time.Sleep(time.Second)
			p.cleanup()
		}
	}
}

func (p *Prober) cleanup() {
	p.targetsMu.Lock()
	defer p.targetsMu.Unlock()

	for _, t := range p.targets {
		p.measurements.RemoveOlder(p.lastFinishedMeasurement(t))
	}
}

func (p *Prober) init() error {
	err := p.initRawSocket()
	if err != nil {
		return fmt.Errorf("unable to initialize RAW socket: %v", err)
	}

	err = p.initUDPSocket()
	if err != nil {
		return fmt.Errorf("unable to initialize UDP socket: %v", err)
	}

	return nil
}
