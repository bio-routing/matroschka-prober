package target

import (
	"fmt"
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

const (
	ttl = 64
)

var (
	ipv4inGRE = &layers.GRE{
		Protocol: layers.EthernetTypeIPv4,
	}
	ipv6inGRE = &layers.GRE{
		Protocol: layers.EthernetTypeIPv6,
	}
)

func (t *Target) getSrcAddrHop(hop int, seq uint64) net.IP {
	return t.cfg.Hops[hop-1].SrcRange[seq%uint64(len(t.cfg.Hops[hop-1].SrcRange))]
}

func (t *Target) getDstAddr(hop int, seq uint64) net.IP {
	return t.cfg.Hops[hop].DstRange[seq%uint64(len(t.cfg.Hops[hop].DstRange))]
}

func (t *Target) CraftPacket(pr Probe, udpPort uint16) ([]byte, error) {
	probeSer := pr.marshal()

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}

	l := make([]gopacket.SerializableLayer, 0, 10)

	var err error
	ipProtocolVersion := t.firstHopAFI()
	if ipProtocolVersion == 4 {
		l, err = t.craftIPV4Packet(pr.SequenceNumber, l, udpPort)
		if err != nil {
			return nil, fmt.Errorf("failed to craft IPv4 packet: %w", err)
		}
	}

	if ipProtocolVersion == 6 {
		l, err = t.craftIPV6Packet(pr.SequenceNumber, l, udpPort)
		if err != nil {
			return nil, fmt.Errorf("failed to craft IPv6 packet: %w", err)
		}
	}

	l = append(l, gopacket.Payload(probeSer[:]))

	err = gopacket.SerializeLayers(buf, opts, l...)
	if err != nil {
		return nil, fmt.Errorf("unable to serialize layers: %v", err)
	}

	return buf.Bytes(), nil
}

func (t *Target) craftIPV4Packet(sequenceNumber uint64, l []gopacket.SerializableLayer, udpPort uint16) ([]gopacket.SerializableLayer, error) {
	l = append(l, ipv4inGRE)

	for i := range t.cfg.Hops {
		if i == 0 {
			continue
		}

		l = append(l, &layers.IPv4{
			SrcIP:    t.getSrcAddrHop(i, sequenceNumber),
			DstIP:    t.getDstAddr(i, sequenceNumber),
			Version:  4,
			Protocol: layers.IPProtocolGRE,
			TOS:      t.cfg.TOS.Value,
			TTL:      ttl,
		})

		l = append(l, ipv4inGRE)
	}

	// Create final UDP packet that will return
	ip := &layers.IPv4{
		SrcIP:    t.getSrcAddrHop(len(t.cfg.Hops), sequenceNumber),
		DstIP:    t.localAddr,
		Version:  4,
		Protocol: layers.IPProtocolUDP,
		TOS:      t.cfg.TOS.Value,
		TTL:      ttl,
	}
	l = append(l, ip)

	udp := &layers.UDP{
		SrcPort: layers.UDPPort(udpPort),
		DstPort: layers.UDPPort(udpPort),
	}

	err := udp.SetNetworkLayerForChecksum(ip)
	if err != nil {
		return nil, fmt.Errorf("couldn't set the network layer for checksum: %w", err)
	}
	l = append(l, udp)

	return l, nil
}

func (t *Target) craftIPV6Packet(sequenceNumber uint64, l []gopacket.SerializableLayer, udpPort uint16) ([]gopacket.SerializableLayer, error) {
	l = append(l, ipv6inGRE)

	for i := range t.cfg.Hops {
		if i == 0 {
			continue
		}

		l = append(l, &layers.IPv6{
			SrcIP:        t.getSrcAddrHop(i, sequenceNumber),
			DstIP:        t.getDstAddr(i, sequenceNumber),
			Version:      6,
			TrafficClass: t.cfg.TOS.Value,
			NextHeader:   layers.IPProtocolGRE,
			HopLimit:     ttl,
		})

		l = append(l, ipv6inGRE)

	}

	ip := &layers.IPv6{
		SrcIP:        t.getSrcAddrHop(len(t.cfg.Hops), sequenceNumber),
		DstIP:        t.localAddr,
		Version:      6,
		TrafficClass: t.cfg.TOS.Value,
		NextHeader:   layers.IPProtocolUDP,
		HopLimit:     ttl,
	}
	l = append(l, ip)

	udp := &layers.UDP{
		SrcPort: layers.UDPPort(udpPort),
		DstPort: layers.UDPPort(udpPort),
	}

	err := udp.SetNetworkLayerForChecksum(ip)
	if err != nil {
		return nil, fmt.Errorf("couldn't set the network layer for checksum: %w", err)
	}
	l = append(l, udp)

	return l, nil
}

func (t *Target) firstHopAFI() int8 {
	if t.cfg.Hops[0].DstRange[0].To4() != nil {
		return 4
	}

	return 6
}
