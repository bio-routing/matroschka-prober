package prober

import (
	"fmt"
	"net"
	"strconv"
	"time"
	"unsafe"

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
	Read([]byte) (int, *time.Time, error)
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
	sockfd int
	port   uint16
}

func newUDPSockWrapper(port uint16, rmem int) (*udpSockWrapper, error) {
	sockfd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, unix.IPPROTO_UDP)
	if err != nil {
		return nil, fmt.Errorf("unable to create UDP socket: %v", err)
	}

	err = unix.SetsockoptInt(sockfd, unix.SOL_SOCKET, unix.SO_TIMESTAMPNS, 1)
	if err != nil {
		return nil, fmt.Errorf("unable to set SO_TIMESTAMP on UDP socket: %v", err)
	}

	if rmem > 0 {
		err = unix.SetsockoptInt(sockfd, unix.SOL_SOCKET, unix.SO_RCVBUF, rmem)
		if err != nil {
			return nil, fmt.Errorf("unable to set UDP socket receive buffer size: %v", err)
		}
	}

	err = unix.Bind(sockfd, &unix.SockaddrInet4{
		Port: int(port),
	})
	if err != nil {
		return nil, fmt.Errorf("unable to bind UDP socket to port %d: %v", port, err)
	}

	return &udpSockWrapper{
		sockfd: sockfd,
		port:   port,
	}, nil
}

func (u *udpSockWrapper) Read(b []byte) (int, *time.Time, error) {
	oob := make([]byte, 1024)
	n, oobn, _, _, err := unix.Recvmsg(u.sockfd, b, oob, 0)
	if err != nil {
		return n, nil, fmt.Errorf("recvmsg failed: %w", err)
	}

	cmsgs, err := unix.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return n, nil, fmt.Errorf("unable to parse socket control message: %v", err)
	}

	var ts *time.Time
	for _, cmsg := range cmsgs {
		if cmsg.Header.Level == unix.SOL_SOCKET && cmsg.Header.Type == unix.SO_TIMESTAMPNS {
			tspecRaw := [16]byte{}
			copy(tspecRaw[:], cmsg.Data[:16])
			tspec := (*unix.Timespec)(unsafe.Pointer(&tspecRaw))
			rxTS := time.Unix(int64(tspec.Sec), int64(tspec.Nsec))
			ts = &rxTS
		}
	}

	return n, ts, nil
}

func (u *udpSockWrapper) Close() error {
	return unix.Close(u.sockfd)
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
	for i := range maxPort {
		s, err := newUDPSockWrapper(p.basePort+uint16(i), p.rmem)
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
