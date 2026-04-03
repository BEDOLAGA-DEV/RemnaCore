package plugin

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// blockedHostnames are DNS names that always resolve to loopback or local
// addresses. Checked case-insensitively before any DNS lookup.
var blockedHostnames = []string{
	"localhost",
	"127.0.0.1",
	"0.0.0.0",
	"[::1]",
	"::1",
}

// blockedCIDRs are private and reserved IP ranges that plugins must never
// reach. This prevents SSRF attacks against the internal Docker/Kubernetes
// network, cloud metadata endpoints, and link-local services.
var blockedCIDRs []*net.IPNet

func init() {
	cidrs := []string{
		"10.0.0.0/8",      // RFC 1918
		"172.16.0.0/12",   // RFC 1918
		"192.168.0.0/16",  // RFC 1918
		"127.0.0.0/8",     // loopback
		"169.254.0.0/16",  // link-local
		"100.64.0.0/10",   // carrier-grade NAT (RFC 6598)
		"::1/128",         // IPv6 loopback
		"fc00::/7",        // IPv6 unique local
		"fe80::/10",       // IPv6 link-local
	}
	for _, cidr := range cidrs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			// CIDRs are compile-time constants; a parse failure is a programming error.
			panic(fmt.Sprintf("invalid blocked CIDR %q: %v", cidr, err))
		}
		blockedCIDRs = append(blockedCIDRs, ipNet)
	}
}

// isBlockedHostname checks whether the hostname portion of a URL is a known
// loopback/local name. This is a fast pre-flight check that does not require
// DNS resolution.
func isBlockedHostname(rawURL string) (bool, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		// Unparseable URLs are rejected.
		return true, fmt.Errorf("unparseable URL: %w", err)
	}

	hostname := u.Hostname()
	if hostname == "" {
		return true, fmt.Errorf("empty hostname in URL %q", rawURL)
	}

	lower := strings.ToLower(hostname)
	for _, blocked := range blockedHostnames {
		if lower == blocked {
			return true, nil
		}
	}
	return false, nil
}

// isPrivateIP returns true if the given IP falls within any of the blocked
// CIDR ranges.
func isPrivateIP(ip net.IP) bool {
	for _, cidr := range blockedCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// ssrfSafeDialContext returns a DialContext function that resolves the target
// address and rejects connections to private/internal IP ranges. This prevents
// DNS rebinding attacks where an allowed hostname resolves to an internal IP.
func ssrfSafeDialContext(baseDialer *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("split host/port %q: %w", addr, err)
		}

		// Resolve the hostname to IP addresses.
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolve %q: %w", host, err)
		}

		if len(ips) == 0 {
			return nil, fmt.Errorf("no addresses found for host %q", host)
		}

		// Check EVERY resolved address, not just the first.
		for _, ipAddr := range ips {
			if isPrivateIP(ipAddr.IP) {
				return nil, fmt.Errorf("%w: host %q resolves to private IP %s",
					ErrInternalNetworkAccess, host, ipAddr.IP)
			}
		}

		// Dial the first resolved address that passed validation.
		target := net.JoinHostPort(ips[0].IP.String(), port)
		return baseDialer.DialContext(ctx, network, target)
	}
}

// SSRFSafeDialTimeout is the default dial timeout for SSRF-safe connections.
const SSRFSafeDialTimeout = 5 * time.Second
