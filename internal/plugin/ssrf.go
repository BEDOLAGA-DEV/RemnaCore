// ssrf.go implements four-layer SSRF (Server-Side Request Forgery) protection
// for outbound HTTP requests made by WASM plugins.
//
// The layers are applied in order by [HostFunctions.HTTPRequest]:
//
//  1. URL path normalization ([normalizeURL]): collapses and rejects path
//     traversal sequences ("..") before any network operation.
//
//  2. Manifest allowlist ([PermissionChecker.ValidateHTTPRequest]): the
//     normalized URL must match at least one pattern in the plugin's manifest
//     HTTP allowlist. Patterns support exact match and wildcard suffix
//     ("https://api.example.com/*").
//
//  3. Pre-flight hostname check ([isBlockedHostname]): a fast, zero-DNS check
//     that rejects URLs whose hostname matches known loopback, private, or
//     cloud metadata addresses (localhost, 127.0.0.1, 169.254.169.254,
//     metadata.google.internal, etc.). The comparison is case-insensitive.
//
//  4. Transport-level dial guard ([ssrfSafeDialContext]): the custom DialContext
//     resolves the target hostname to IP addresses, then checks EVERY resolved
//     IP against [blockedCIDRs] (RFC 1918, loopback, link-local, CGNAT, IPv6
//     ULA/link-local). If any IP is private, the connection is rejected. The
//     actual TCP connection is made to the resolved IP, not the original
//     hostname, eliminating DNS rebinding attacks where a second resolution
//     returns a different (internal) IP.
//
// These layers are independent: even if a plugin's manifest allowlist includes
// "http://localhost:3000/*", layers 3 and 4 will still reject the request. This
// ensures that a misconfigured allowlist cannot bypass network security.
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
// addresses. Checked case-insensitively before any DNS lookup. Cloud metadata
// hostnames are included to prevent SSRF attacks that bypass CIDR checks via
// DNS resolution.
var blockedHostnames = []string{
	"localhost",
	"127.0.0.1",
	"0.0.0.0",
	"[::1]",
	"::1",
	// Cloud metadata endpoints — DNS-level block before resolution.
	"metadata.google.internal", // GCP metadata
	"metadata.goog",           // GCP alternative
	"169.254.169.254",         // AWS/GCP/Azure metadata IP
	"fd00:ec2::254",           // AWS IPv6 metadata
	"metadata",                // bare hostname used in some GCP environments
	"instance-data",           // DigitalOcean metadata
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
