package plugin

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsBlockedHostname(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		blocked bool
		wantErr bool
	}{
		{
			name:    "localhost",
			rawURL:  "http://localhost:3000/api",
			blocked: true,
		},
		{
			name:    "127.0.0.1",
			rawURL:  "http://127.0.0.1:4000/api",
			blocked: true,
		},
		{
			name:    "0.0.0.0",
			rawURL:  "http://0.0.0.0:8080/",
			blocked: true,
		},
		{
			name:    "ipv6 loopback bracket notation",
			rawURL:  "http://[::1]:8080/",
			blocked: true,
		},
		{
			name:    "LOCALHOST uppercase",
			rawURL:  "http://LOCALHOST/admin",
			blocked: true,
		},
		{
			name:    "external API",
			rawURL:  "https://api.stripe.com/v1/charges",
			blocked: false,
		},
		{
			name:    "external example",
			rawURL:  "https://example.com/api",
			blocked: false,
		},
		{
			name:    "empty URL",
			rawURL:  "",
			blocked: true,
			wantErr: true,
		},
		{
			name:    "invalid URL",
			rawURL:  "://not-a-url",
			blocked: true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocked, err := isBlockedHostname(tt.rawURL)
			if tt.wantErr {
				assert.Error(t, err)
			}
			assert.Equal(t, tt.blocked, blocked)
		})
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		private bool
	}{
		// RFC 1918 - 10.0.0.0/8
		{name: "10.0.0.1", ip: "10.0.0.1", private: true},
		{name: "10.255.255.255", ip: "10.255.255.255", private: true},

		// RFC 1918 - 172.16.0.0/12
		{name: "172.16.0.1", ip: "172.16.0.1", private: true},
		{name: "172.18.0.5 (Docker)", ip: "172.18.0.5", private: true},
		{name: "172.31.255.255", ip: "172.31.255.255", private: true},
		{name: "172.15.255.255 (not private)", ip: "172.15.255.255", private: false},
		{name: "172.32.0.0 (not private)", ip: "172.32.0.0", private: false},

		// RFC 1918 - 192.168.0.0/16
		{name: "192.168.1.1", ip: "192.168.1.1", private: true},
		{name: "192.168.0.0", ip: "192.168.0.0", private: true},

		// Loopback - 127.0.0.0/8
		{name: "127.0.0.1", ip: "127.0.0.1", private: true},
		{name: "127.0.0.2", ip: "127.0.0.2", private: true},

		// Link-local - 169.254.0.0/16 (cloud metadata)
		{name: "169.254.169.254 (metadata)", ip: "169.254.169.254", private: true},
		{name: "169.254.0.1", ip: "169.254.0.1", private: true},

		// Carrier-grade NAT - 100.64.0.0/10 (RFC 6598)
		{name: "100.64.0.1", ip: "100.64.0.1", private: true},
		{name: "100.127.255.255", ip: "100.127.255.255", private: true},
		{name: "100.128.0.0 (not CGNAT)", ip: "100.128.0.0", private: false},

		// Public IPs
		{name: "8.8.8.8 (public)", ip: "8.8.8.8", private: false},
		{name: "1.1.1.1 (public)", ip: "1.1.1.1", private: false},
		{name: "93.184.216.34 (public)", ip: "93.184.216.34", private: false},

		// IPv6
		{name: "::1 (IPv6 loopback)", ip: "::1", private: true},
		{name: "fc00::1 (IPv6 ULA)", ip: "fc00::1", private: true},
		{name: "fe80::1 (IPv6 link-local)", ip: "fe80::1", private: true},
		{name: "2001:db8::1 (IPv6 public)", ip: "2001:db8::1", private: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			require.NotNil(t, ip, "failed to parse IP %q", tt.ip)
			assert.Equal(t, tt.private, isPrivateIP(ip))
		})
	}
}

func TestBlockedCIDRsInitialized(t *testing.T) {
	// Sanity check: the init() function should have populated blockedCIDRs.
	expectedMinCIDRs := 9
	assert.GreaterOrEqual(t, len(blockedCIDRs), expectedMinCIDRs,
		"expected at least %d blocked CIDRs, got %d", expectedMinCIDRs, len(blockedCIDRs))
}
