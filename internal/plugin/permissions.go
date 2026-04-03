package plugin

import "strings"

// PermissionChecker validates plugin permission scopes against the manifest
// declarations. It is safe for concurrent use (stateless).
type PermissionChecker struct{}

// HasPermission checks whether a plugin has been granted a specific permission
// scope. Write scopes implicitly grant the corresponding read scope.
func (pc *PermissionChecker) HasPermission(p *Plugin, scope PermissionScope) bool {
	for _, granted := range p.Permissions {
		if granted == scope {
			return true
		}
		if impliesPermission(granted, scope) {
			return true
		}
	}
	return false
}

// ValidateHTTPRequest checks whether a plugin is allowed to make HTTP requests
// to the given URL by matching against the manifest's http allowlist patterns.
// Patterns support a trailing wildcard (e.g. "https://api.stripe.com/*").
func (pc *PermissionChecker) ValidateHTTPRequest(p *Plugin, url string) bool {
	if p.Manifest == nil {
		return false
	}
	for _, pattern := range p.Manifest.Permissions.HTTP {
		if matchHTTPPattern(pattern, url) {
			return true
		}
	}
	return false
}

// impliesPermission returns true when the granted scope implicitly covers the
// requested scope. The rule is: a "write" scope implies "read" for the same
// resource, and "readwrite" implies "read".
func impliesPermission(granted, requested PermissionScope) bool {
	grantedStr := string(granted)
	requestedStr := string(requested)

	// Extract resource prefix (everything before the colon).
	gParts := strings.SplitN(grantedStr, ":", 2)
	rParts := strings.SplitN(requestedStr, ":", 2)

	if len(gParts) != 2 || len(rParts) != 2 {
		return false
	}

	// Must be the same resource.
	if gParts[0] != rParts[0] {
		return false
	}

	// "write" implies "read"; "readwrite" implies "read".
	if rParts[1] == "read" && (gParts[1] == "write" || gParts[1] == "readwrite") {
		return true
	}

	return false
}

// matchHTTPPattern matches a URL against an allowlist pattern. Supported forms:
//   - Exact match: "https://api.example.com/v1/webhook"
//   - Wildcard suffix: "https://api.example.com/*" matches any path under that host.
func matchHTTPPattern(pattern, url string) bool {
	if pattern == url {
		return true
	}
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		// URL must start with the prefix and either equal it or continue with "/".
		if url == prefix {
			return true
		}
		if strings.HasPrefix(url, prefix+"/") {
			return true
		}
	}
	return false
}
