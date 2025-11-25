package prober

import (
	"fmt"
	"net"
	"strconv"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"golang.org/x/sys/unix"
)

const ttl = 64
const maxPort = 65535

type rawSocket interface {
	WriteTo(payload []byte, options writeOptions) error
	Close() error
}

type writeOptions struct {
	src      net.IP
	dst      net.IP
	tos      int64
	ttl      int64
	protocol int64
}

type udpSocket interface {
	Read([]byte) (int, error)
	Close() error
}

type rawSockWrapper struct {
	rawConn *ipv4.RawConn
}

func newRawSockWrapper() (*rawSockWrapper, error) {
	greProtoStr := strconv.FormatInt(unix.IPPROTO_GRE, 10)
	c, err := net.ListenPacket("ip4:"+greProtoStr, "0.0.0.0") // GRE for IPv4
	if err != nil {
		return nil, fmt.Errorf("unable to listen for GRE packets: %v", err)
	}

	rc, err := ipv4.NewRawConn(c)
	if err != nil {
		return nil, fmt.Errorf("unable to create raw connection: %v", err)
	}

	return &rawSockWrapper{
		rawConn: rc,
	}, nil
}

func (s *rawSockWrapper) WriteTo(p []byte, o writeOptions) error {

	iph := &ipv4.Header{
		Src:      o.src,
		Dst:      o.dst,
		Version:  ipv4.Version,
		Len:      ipv4.HeaderLen,
		TOS:      int(o.tos),
		TotalLen: ipv4.HeaderLen + len(p),
		TTL:      ttl,
		Protocol: unix.IPPROTO_GRE,
	}
	cm := &ipv4.ControlMessage{}
	if o.src != nil {
		cm.Src = o.src
	}

	return s.rawConn.WriteTo(iph, p, cm)
}

func (s *rawSockWrapper) Close() error {
	return s.rawConn.Close()
}

type udpSockWrapper struct {
	udpConn *net.UDPConn
	port    uint16
}

func newUDPSockWrapper(port uint16) (*udpSockWrapper, error) {
	var udpConn *net.UDPConn

	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("unable to resolve address: %v", err)
	}

	udpConn, err = net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, fmt.Errorf("unable to listen for UDP packets: %v", err)

	}

	return &udpSockWrapper{
		udpConn: udpConn,
		port:    port,
	}, nil
}

func (u *udpSockWrapper) getPort() uint16 {
	return u.port
}

func (u *udpSockWrapper) Read(b []byte) (int, error) {
	return u.udpConn.Read(b)
}

func (u *udpSockWrapper) Close() error {
	return u.udpConn.Close()
}

func (p *Prober) initRawSocket() error {
	rc4, err := newRawSockWrapper()
	if err != nil {
		return fmt.Errorf("unable to create rack socket wrapper: %v", err)
	}

	p.rawConn4 = rc4

	rc6, err := newIPv6RawSockWrapper()
	if err != nil {
		return fmt.Errorf("unable to create rack socket wrapper: %v", err)
	}

	p.rawConn6 = rc6

	return nil
}

func (p *Prober) initUDPSocket() error {
	for i := 0; i < maxPort; i++ {
		s, err := newUDPSockWrapper(p.basePort + uint16(i))
		if err != nil {
			continue
		}

		p.udpPort = p.basePort + uint16(i)
		p.udpConn = s
		return nil
	}

	return fmt.Errorf("unable to find free UDP port")
}

type rawIPv6SocketWrapper struct {
	rawIPv6Conn *ipv6.PacketConn
}

func (s *rawIPv6SocketWrapper) WriteTo(p []byte, o writeOptions) error {
	cm := &ipv6.ControlMessage{
		TrafficClass: int(o.tos),
		HopLimit:     ttl,
		Src:          o.src,
		Dst:          o.dst,
	}

	dstAddress := net.IPAddr{IP: o.dst}

	_, err := s.rawIPv6Conn.WriteTo(p, cm, &dstAddress)
	return err
}

func (s *rawIPv6SocketWrapper) Close() error {
	return s.rawIPv6Conn.Close()
}

func newIPv6RawSockWrapper() (*rawIPv6SocketWrapper, error) {
	greProtoStr := strconv.FormatInt(unix.IPPROTO_GRE, 10)
	c, err := net.ListenPacket("ip6:"+greProtoStr, "::")
	if err != nil {
		return nil, fmt.Errorf("unable to listen for GRE packets: %v", err)
	}

	rc := ipv6.NewPacketConn(c)

	return &rawIPv6SocketWrapper{
		rawIPv6Conn: rc,
	}, nil
}
