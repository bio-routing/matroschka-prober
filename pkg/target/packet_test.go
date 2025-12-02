package target

import (
	"net"
	"testing"

	"github.com/bio-routing/matroschka-prober/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestTarget_CraftPacket(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		cfg        TargetConfig
		returnAddr net.IP
		// Named input parameters for target function.
		pr       Probe
		udpPort  uint16
		expected []byte
		wantErr  bool
	}{
		{
			name: "basic test ipv4",
			cfg: TargetConfig{
				Name: "test-target",
				TOS:  TOS{Value: 0},
				Hops: []config.Hop{
					{
						SrcRange: []net.IP{net.ParseIP("192.0.2.0")},
						DstRange: []net.IP{net.ParseIP("169.254.0.0")},
					},
				},
				SrcAddrs:            []net.IP{net.ParseIP("192.0.2.0")},
				MeasurementLengthMS: 1000,
				TimeoutMS:           500,
			},
			returnAddr: net.ParseIP("128.0.0.1"),
			pr: Probe{
				SequenceNumber:    1,
				TimeStampUnixNano: 123456789,
			},
			udpPort: 33434,
			expected: []byte{
				0x0, 0x0, 0x8, 0x0, 0x45, 0x0, 0x0, 0x2c, 0x0, 0x0, 0x0, 0x0, 0x40, 0x11, 0x38, 0xc0, 0xc0, 0x0, 0x2, 0x0, 0x80, 0x0, 0x0, 0x1, 0x82, 0x9a, 0x82, 0x9a, 0x0, 0x18, 0xe4, 0x15, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x0, 0x7, 0x5b, 0xcd, 0x15,
			},
			wantErr: false,
		},
		{
			name: "basic test ipv6",
			cfg: TargetConfig{
				Name: "test-target",
				TOS:  TOS{Value: 0},
				Hops: []config.Hop{
					{
						SrcRange: []net.IP{net.ParseIP("2001:db8::1")},
						DstRange: []net.IP{net.ParseIP("2001:db8::2")},
					},
				},
				SrcAddrs:            []net.IP{net.ParseIP("2001:db8::1")},
				MeasurementLengthMS: 1000,
				TimeoutMS:           500,
			},
			returnAddr: net.ParseIP("2001:db8::ff"),
			pr: Probe{
				SequenceNumber:    1,
				TimeStampUnixNano: 123456789,
			},
			udpPort: 33434,
			expected: []byte{
				0x0, 0x0, 0x86, 0xdd, 0x60, 0x0, 0x0, 0x0, 0x0, 0x18, 0x11, 0x40, 0x20, 0x1, 0xd, 0xb8, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1, 0x20, 0x1, 0xd, 0xb8, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xff, 0x82, 0x9a, 0x82, 0x9a, 0x0, 0x18, 0xc9, 0xa5, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x0, 0x7, 0x5b, 0xcd, 0x15,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ta, err := NewTarget(tt.cfg, tt.returnAddr)
			if err != nil {
				t.Fatalf("could not construct receiver type: %v", err)
			}
			got, gotErr := ta.CraftPacket(tt.pr, tt.udpPort)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("CraftPacket() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("CraftPacket() succeeded unexpectedly")
			}

			assert.Equalf(t, tt.expected, got, tt.name)
		})
	}
}
