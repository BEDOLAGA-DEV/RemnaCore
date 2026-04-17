package remnawave

import (
	"net/url"
)

// Client app deep link builders.
var clientURLBuilders = map[string]func(subURL string) string{
	"v2raytun": func(subURL string) string {
		return "v2raytun://import/" + subURL
	},
	"happ": func(subURL string) string {
		return "happ://add/" + subURL
	},
	"streisand": func(subURL string) string {
		return "streisand://import/" + subURL
	},
	"singbox": func(subURL string) string {
		return "sing-box://import-remote-profile?url=" + url.QueryEscape(subURL)
	},
	"karing": func(subURL string) string {
		return "karing://install-config?url=" + url.QueryEscape(subURL)
	},
}

// BuildSubscriptionURL constructs subscription URLs for various client apps.
func BuildSubscriptionURL(rawSubURL, basePageURL, preferredClient string) SubscriptionURL {
	subURL := rawSubURL
	if basePageURL != "" && rawSubURL != "" {
		// If we have a custom page URL, use it instead of the raw panel URL
		subURL = basePageURL + "/" + extractSubPath(rawSubURL)
	}

	result := SubscriptionURL{
		BaseURL:    subURL,
		ClientURLs: make(map[string]string),
	}

	if subURL == "" {
		return result
	}

	// Build deep links for all supported clients
	for appName, builder := range clientURLBuilders {
		result.ClientURLs[appName] = builder(subURL)
	}

	// Set preferred deep link
	if preferredClient != "" && preferredClient != "auto" {
		if link, ok := result.ClientURLs[preferredClient]; ok {
			result.DeepLink = link
		}
	}

	return result
}

// extractSubPath gets the path component from a subscription URL.
func extractSubPath(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if u.Path == "" {
		return rawURL
	}
	// Return just the last path segment (the subscription token)
	path := u.Path
	if len(path) > 1 && path[0] == '/' {
		path = path[1:]
	}
	return path
}

